package linker

import (
	"fmt"
	"regexp"
	"sort"
)

var (
	identityRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*$`)
	releaseRE  = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	digestRE   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	symbolRE   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func Link(policy Policy, input []Module) (LinkedGraph, error) {
	if err := validatePolicy(policy); err != nil {
		return LinkedGraph{}, err
	}
	modules := append([]Module(nil), input...)
	for i := range modules {
		if err := validateModule(modules[i]); err != nil {
			return LinkedGraph{}, fmt.Errorf("module %q: %w", modules[i].Identity, err)
		}
		sortModule(&modules[i])
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Identity < modules[j].Identity })

	graph := LinkedGraph{Schema: "gooo.linked-ir/v1", Status: policy.Default, Modules: modules}
	byIdentity := map[string]Module{}
	for _, module := range modules {
		if _, exists := byIdentity[module.Identity]; exists {
			addRuleDiagnostic(&graph, policy, "duplicate_module", module.Identity, "", "", "module identity appears more than once")
			continue
		}
		byIdentity[module.Identity] = module
		if ModuleDigest(module) != module.Digest {
			addRuleDiagnostic(&graph, policy, "module_digest_mismatch", module.Identity, module.Identity, "", "declared module digest differs from canonical declaration")
		}
		for _, export := range module.Exports {
			if export.Owner != module.Identity {
				addRuleDiagnostic(&graph, policy, "ownership_mismatch", module.Identity, module.Identity, export.Symbol, "export owner does not match declaring module identity")
			}
		}
	}

	owners := map[string][]string{}
	for _, module := range modules {
		for _, export := range module.Exports {
			owners[export.Symbol] = append(owners[export.Symbol], module.Identity)
		}
	}
	for symbol, identities := range owners {
		if len(identities) > 1 {
			sort.Strings(identities)
			addRuleDiagnostic(&graph, policy, "duplicate_export", stringsJoin(identities), "", symbol, "exported symbol has more than one owner")
		}
	}

	adjacency := map[string][]string{}
	for _, module := range modules {
		adjacency[module.Identity] = nil
		for _, imp := range module.Imports {
			target, exists := byIdentity[imp.Module]
			if !exists {
				addUnknownDiagnostic(&graph, policy, "missing_import", module.Identity, imp.Module, imp.Symbol, "immutable dependency release is not present")
				continue
			}
			adjacency[module.Identity] = append(adjacency[module.Identity], imp.Module)
			graph.Edges = append(graph.Edges, Edge{From: module.Identity, To: imp.Module, Release: imp.Release, Digest: imp.Digest, Symbol: imp.Symbol, Owner: imp.Owner})
			if imp.Release != target.Release {
				addRuleDiagnostic(&graph, policy, "release_mismatch", module.Identity, imp.Module, imp.Symbol, "import requires an exact release identity")
			}
			if imp.Digest != target.Digest {
				addRuleDiagnostic(&graph, policy, "digest_mismatch", module.Identity, imp.Module, imp.Symbol, "import digest does not match the immutable dependency digest")
			}
			if imp.Owner != target.Identity {
				addRuleDiagnostic(&graph, policy, "ownership_mismatch", module.Identity, imp.Module, imp.Symbol, "import owner does not match target module identity")
			}
			if !exportsSymbol(target, imp.Symbol) {
				addUnknownDiagnostic(&graph, policy, "missing_import", module.Identity, imp.Module, imp.Symbol, "requested symbol is not exported by the exact dependency")
			}
		}
		sort.Strings(adjacency[module.Identity])
	}
	sort.Slice(graph.Edges, func(i, j int) bool {
		left := graph.Edges[i].From + "\x00" + graph.Edges[i].To + "\x00" + graph.Edges[i].Symbol + "\x00" + graph.Edges[i].Release + "\x00" + graph.Edges[i].Digest
		right := graph.Edges[j].From + "\x00" + graph.Edges[j].To + "\x00" + graph.Edges[j].Symbol + "\x00" + graph.Edges[j].Release + "\x00" + graph.Edges[j].Digest
		return left < right
	})
	for _, component := range cycleComponents(adjacency) {
		codes := map[string]bool{}
		for _, identity := range component {
			mode := byIdentity[identity].CyclePolicy
			if code, ok := policy.CycleRules[mode]; ok {
				codes[code] = true
			} else {
				return LinkedGraph{}, fmt.Errorf("module %q uses undeclared cycle policy %q", identity, mode)
			}
		}
		code := highestRule(policy, codes)
		rule, err := policy.RuleFor(code)
		if err != nil {
			return LinkedGraph{}, err
		}
		addDiagnostic(&graph, rule, Diagnostic{Code: code, Status: rule.Outcome, Module: stringsJoin(component), Reason: rule.Reason, Unknown: rule.UnknownEvidence(stringsJoin(component))})
	}
	sortDiagnostics(graph.Diagnostics)
	graph.Status = highestStatus(policy, policy.Default, graph.Diagnostics)
	digest, err := CanonicalGraphDigest(graph)
	if err != nil {
		return LinkedGraph{}, err
	}
	graph.CanonicalDigest = digest
	return graph, nil
}

func validateModule(module Module) error {
	if !identityRE.MatchString(module.Identity) {
		return fmt.Errorf("invalid module identity")
	}
	if !releaseRE.MatchString(module.Release) {
		return fmt.Errorf("release must be an exact vX.Y.Z identity")
	}
	if !digestRE.MatchString(module.Digest) {
		return fmt.Errorf("digest must be sha256 followed by 64 lowercase hex characters")
	}
	if module.CyclePolicy == "" {
		return fmt.Errorf("cycle policy is required")
	}
	for _, export := range module.Exports {
		if !symbolRE.MatchString(export.Symbol) || !identityRE.MatchString(export.Owner) {
			return fmt.Errorf("invalid export declaration")
		}
	}
	for _, imp := range module.Imports {
		if !identityRE.MatchString(imp.Module) || !releaseRE.MatchString(imp.Release) || !digestRE.MatchString(imp.Digest) || !symbolRE.MatchString(imp.Symbol) || !identityRE.MatchString(imp.Owner) {
			return fmt.Errorf("invalid import declaration")
		}
	}
	return nil
}

func exportsSymbol(module Module, symbol string) bool {
	for _, export := range module.Exports {
		if export.Symbol == symbol {
			return true
		}
	}
	return false
}

func addRuleDiagnostic(graph *LinkedGraph, policy Policy, code, module, target, symbol, fallback string) {
	rule, err := policy.RuleFor(code)
	if err != nil {
		graph.Diagnostics = append(graph.Diagnostics, Diagnostic{Code: "policy_error", Status: "REFUTED", Module: module, Reason: err.Error()})
		return
	}
	reason := rule.Reason
	if reason == "" {
		reason = fallback
	}
	addDiagnostic(graph, rule, Diagnostic{Code: code, Status: rule.Outcome, Module: module, Target: target, Symbol: symbol, Reason: reason, Unknown: rule.UnknownEvidence(module + " -> " + target)})
}

func addUnknownDiagnostic(graph *LinkedGraph, policy Policy, code, module, target, symbol, fallback string) {
	addRuleDiagnostic(graph, policy, code, module, target, symbol, fallback)
}

func addDiagnostic(graph *LinkedGraph, rule Rule, diagnostic Diagnostic) {
	diagnostic.Status = rule.Outcome
	if diagnostic.Reason == "" {
		diagnostic.Reason = rule.Reason
	}
	if diagnostic.Status != "UNKNOWN" {
		diagnostic.Unknown = nil
	}
	graph.Diagnostics = append(graph.Diagnostics, diagnostic)
}

func highestStatus(policy Policy, initial string, diagnostics []Diagnostic) string {
	best := initial
	for _, diagnostic := range diagnostics {
		if statusRank(policy, diagnostic.Status) < statusRank(policy, best) {
			best = diagnostic.Status
		}
	}
	return best
}

func highestRule(policy Policy, codes map[string]bool) string {
	selected := ""
	for code := range codes {
		rule, ok := policy.Rules[code]
		if !ok {
			continue
		}
		if selected == "" || statusRank(policy, rule.Outcome) < statusRank(policy, policy.Rules[selected].Outcome) || (statusRank(policy, rule.Outcome) == statusRank(policy, policy.Rules[selected].Outcome) && code < selected) {
			selected = code
		}
	}
	return selected
}

func statusRank(policy Policy, status string) int {
	for index, candidate := range policy.Precedence {
		if candidate == status {
			return index
		}
	}
	return len(policy.Precedence)
}

func sortDiagnostics(diagnostics []Diagnostic) {
	sort.Slice(diagnostics, func(i, j int) bool {
		left := diagnostics[i].Code + "\x00" + diagnostics[i].Module + "\x00" + diagnostics[i].Target + "\x00" + diagnostics[i].Symbol + "\x00" + diagnostics[i].Reason
		right := diagnostics[j].Code + "\x00" + diagnostics[j].Module + "\x00" + diagnostics[j].Target + "\x00" + diagnostics[j].Symbol + "\x00" + diagnostics[j].Reason
		return left < right
	})
}

func cycleComponents(adjacency map[string][]string) [][]string {
	index := 0
	indices := map[string]int{}
	lowlink := map[string]int{}
	onStack := map[string]bool{}
	stack := []string{}
	components := [][]string{}
	var visit func(string)
	visit = func(node string) {
		indices[node] = index
		lowlink[node] = index
		index++
		stack = append(stack, node)
		onStack[node] = true
		neighbors := append([]string(nil), adjacency[node]...)
		sort.Strings(neighbors)
		for _, next := range neighbors {
			if _, seen := indices[next]; !seen {
				visit(next)
				if lowlink[next] < lowlink[node] {
					lowlink[node] = lowlink[next]
				}
			} else if onStack[next] && indices[next] < lowlink[node] {
				lowlink[node] = indices[next]
			}
		}
		if lowlink[node] == indices[node] {
			component := []string{}
			for {
				last := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[last] = false
				component = append(component, last)
				if last == node {
					break
				}
			}
			sort.Strings(component)
			if len(component) > 1 || hasSelfLoop(component[0], adjacency) {
				components = append(components, component)
			}
		}
	}
	nodes := make([]string, 0, len(adjacency))
	for node := range adjacency {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	for _, node := range nodes {
		if _, seen := indices[node]; !seen {
			visit(node)
		}
	}
	sort.Slice(components, func(i, j int) bool { return stringsJoin(components[i]) < stringsJoin(components[j]) })
	return components
}

func hasSelfLoop(node string, adjacency map[string][]string) bool {
	for _, next := range adjacency[node] {
		if next == node {
			return true
		}
	}
	return false
}

func stringsJoin(values []string) string {
	result := ""
	for index, value := range values {
		if index > 0 {
			result += ","
		}
		result += value
	}
	return result
}
