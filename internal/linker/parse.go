package linker

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	moduleHeaderRE = regexp.MustCompile(`^module "([^"]+)" release "([^"]+)" digest "([^"]+)" cycle_policy "([^"]+)" \{$`)
	exportRE       = regexp.MustCompile(`^export "([^"]+)" owner "([^"]+)"$`)
	importRE       = regexp.MustCompile(`^import "([^"]+)" release "([^"]+)" digest "([^"]+)" symbol "([^"]+)" owner "([^"]+)"$`)
	policyHeaderRE = regexp.MustCompile(`^policy "([^"]+)" \{$`)
	defaultRE      = regexp.MustCompile(`^default "([^"]+)"$`)
	precedenceRE   = regexp.MustCompile(`^precedence((?: "[^"]+")+)$`)
	ruleRE         = regexp.MustCompile(`^rule "([^"]+)" outcome "([^"]+)" stage "([^"]+)" step "([^"]+)" reason "([^"]+)"(?: unknown_class "([^"]+)" next_operation "([^"]+)")?$`)
	cycleRE        = regexp.MustCompile(`^cycle_policy "([^"]+)" rule "([^"]+)"$`)
	generationRE   = regexp.MustCompile(`^generation "([^"]+)" package "([^"]+)" entrypoint "([^"]+)"$`)
	confHeaderRE   = regexp.MustCompile(`^conformance "([^"]+)" \{$`)
	scenarioRE     = regexp.MustCompile(`^scenario "([^"]+)" expect "([^"]+)" inputs "([^"]+)" selected "(true|false)"$`)
	compareRE      = regexp.MustCompile(`^comparison "([^"]+)" expect "([^"]+)" left "([^"]+)" right "([^"]+)" selected "(true|false)"$`)
)

func cleanLine(raw string) string {
	line := strings.TrimSpace(raw)
	if i := strings.Index(line, "#"); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	return line
}

func readLines(path string, fn func(int, string) error) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		if line := cleanLine(scanner.Text()); line != "" {
			if err := fn(lineNo, line); err != nil {
				return fmt.Errorf("%s:%d: %w", path, lineNo, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func ParseModuleFile(path string) (Module, error) {
	var module Module
	inBody := false
	seenHeader := false
	err := readLines(path, func(_ int, line string) error {
		if !seenHeader {
			match := moduleHeaderRE.FindStringSubmatch(line)
			if match == nil {
				return fmt.Errorf("expected module header")
			}
			module = Module{Identity: match[1], Release: match[2], Digest: match[3], CyclePolicy: match[4]}
			seenHeader = true
			inBody = true
			return nil
		}
		if line == "}" {
			if !inBody {
				return fmt.Errorf("unexpected closing brace")
			}
			inBody = false
			return nil
		}
		if !inBody {
			return fmt.Errorf("content after module body")
		}
		if match := exportRE.FindStringSubmatch(line); match != nil {
			module.Exports = append(module.Exports, Export{Symbol: match[1], Owner: match[2]})
			return nil
		}
		if match := importRE.FindStringSubmatch(line); match != nil {
			module.Imports = append(module.Imports, Import{Module: match[1], Release: match[2], Digest: match[3], Symbol: match[4], Owner: match[5]})
			return nil
		}
		return fmt.Errorf("unknown module clause %q", line)
	})
	if err != nil {
		return Module{}, err
	}
	if !seenHeader || inBody {
		return Module{}, fmt.Errorf("%s: incomplete module declaration", path)
	}
	return module, nil
}

func ParsePolicyFile(path string) (Policy, error) {
	policy := Policy{Rules: map[string]Rule{}, CycleRules: map[string]string{}}
	inBody := false
	seenHeader := false
	err := readLines(path, func(_ int, line string) error {
		if !seenHeader {
			match := policyHeaderRE.FindStringSubmatch(line)
			if match == nil {
				return fmt.Errorf("expected policy header")
			}
			policy.Identity = match[1]
			seenHeader = true
			inBody = true
			return nil
		}
		if line == "}" {
			if !inBody {
				return fmt.Errorf("unexpected closing brace")
			}
			inBody = false
			return nil
		}
		if !inBody {
			return fmt.Errorf("content after policy body")
		}
		if match := defaultRE.FindStringSubmatch(line); match != nil {
			policy.Default = match[1]
			return nil
		}
		if match := precedenceRE.FindStringSubmatch(line); match != nil {
			values, err := quotedFields(match[1])
			if err != nil {
				return err
			}
			policy.Precedence = values
			return nil
		}
		if match := ruleRE.FindStringSubmatch(line); match != nil {
			if _, exists := policy.Rules[match[1]]; exists {
				return fmt.Errorf("duplicate policy rule %q", match[1])
			}
			policy.Rules[match[1]] = Rule{Code: match[1], Outcome: match[2], Stage: match[3], Step: match[4], Reason: match[5], UnknownClass: match[6], NextOperation: match[7]}
			return nil
		}
		if match := cycleRE.FindStringSubmatch(line); match != nil {
			policy.CycleRules[match[1]] = match[2]
			return nil
		}
		if match := generationRE.FindStringSubmatch(line); match != nil {
			policy.Generation = GenerationPlan{Language: match[1], Package: match[2], Entrypoint: match[3]}
			return nil
		}
		return fmt.Errorf("unknown policy clause %q", line)
	})
	if err != nil {
		return Policy{}, err
	}
	if !seenHeader || inBody {
		return Policy{}, fmt.Errorf("%s: incomplete policy declaration", path)
	}
	if err := validatePolicy(policy); err != nil {
		return Policy{}, fmt.Errorf("%s: %w", path, err)
	}
	return policy, nil
}

func ParseConformanceFile(path string) (Conformance, error) {
	var conformance Conformance
	inBody := false
	seenHeader := false
	err := readLines(path, func(_ int, line string) error {
		if !seenHeader {
			match := confHeaderRE.FindStringSubmatch(line)
			if match == nil {
				return fmt.Errorf("expected conformance header")
			}
			conformance.Identity = match[1]
			seenHeader = true
			inBody = true
			return nil
		}
		if line == "}" {
			inBody = false
			return nil
		}
		if !inBody {
			return fmt.Errorf("content after conformance body")
		}
		if match := scenarioRE.FindStringSubmatch(line); match != nil {
			conformance.Scenarios = append(conformance.Scenarios, Scenario{Name: match[1], Expected: match[2], Inputs: csv(match[3]), Selected: match[4] == "true"})
			return nil
		}
		if match := compareRE.FindStringSubmatch(line); match != nil {
			conformance.Comparisons = append(conformance.Comparisons, Comparison{Name: match[1], Expected: match[2], Left: csv(match[3]), Right: csv(match[4]), Selected: match[5] == "true"})
			return nil
		}
		return fmt.Errorf("unknown conformance clause %q", line)
	})
	if err != nil {
		return Conformance{}, err
	}
	if !seenHeader || inBody {
		return Conformance{}, fmt.Errorf("%s: incomplete conformance declaration", path)
	}
	if len(conformance.Scenarios) == 0 && len(conformance.Comparisons) == 0 {
		return Conformance{}, fmt.Errorf("%s: conformance denominator is empty", path)
	}
	return conformance, nil
}

func validatePolicy(policy Policy) error {
	if policy.Identity == "" || policy.Default == "" || len(policy.Precedence) == 0 {
		return fmt.Errorf("policy must declare identity, default, and precedence")
	}
	seen := map[string]bool{}
	for _, status := range policy.Precedence {
		if seen[status] {
			return fmt.Errorf("duplicate status in precedence %q", status)
		}
		seen[status] = true
	}
	if !seen[policy.Default] {
		return fmt.Errorf("default status %q is absent from precedence", policy.Default)
	}
	for code, rule := range policy.Rules {
		if !seen[rule.Outcome] {
			return fmt.Errorf("rule %q outcome %q is absent from precedence", code, rule.Outcome)
		}
		if rule.Outcome == "UNKNOWN" && (rule.Stage == "" || rule.Step == "" || rule.Reason == "" || rule.UnknownClass == "" || rule.NextOperation == "") {
			return fmt.Errorf("UNKNOWN rule %q must declare stage, step, reason, unknown_class, and next_operation", code)
		}
	}
	if policy.Generation.Language != "go" || policy.Generation.Package == "" || policy.Generation.Entrypoint == "" {
		return fmt.Errorf("policy must declare a Go generation plan")
	}
	for mode, code := range policy.CycleRules {
		if _, ok := policy.Rules[code]; !ok {
			return fmt.Errorf("cycle policy %q references missing rule %q", mode, code)
		}
	}
	return nil
}

func quotedFields(value string) ([]string, error) {
	var values []string
	for len(strings.TrimSpace(value)) > 0 {
		value = strings.TrimSpace(value)
		if !strings.HasPrefix(value, "\"") {
			return nil, fmt.Errorf("expected quoted value in %q", value)
		}
		end := strings.Index(value[1:], "\"")
		if end < 0 {
			return nil, fmt.Errorf("unterminated quoted value")
		}
		end++
		decoded, err := strconv.Unquote(value[:end+1])
		if err != nil {
			return nil, err
		}
		values = append(values, decoded)
		value = value[end+1:]
	}
	return values, nil
}

func csv(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}
