package linker

import "fmt"

type Export struct {
	Symbol string `json:"symbol"`
	Owner  string `json:"owner"`
}

type Import struct {
	Module  string `json:"module"`
	Release string `json:"release"`
	Digest  string `json:"digest"`
	Symbol  string `json:"symbol"`
	Owner   string `json:"owner"`
}

type Module struct {
	Identity    string   `json:"identity"`
	Release     string   `json:"release"`
	Digest      string   `json:"digest"`
	CyclePolicy string   `json:"cycle_policy"`
	Exports     []Export `json:"exports"`
	Imports     []Import `json:"imports"`
}

type Rule struct {
	Code          string `json:"code"`
	Outcome       string `json:"outcome"`
	Stage         string `json:"stage"`
	Step          string `json:"step"`
	Reason        string `json:"reason"`
	UnknownClass  string `json:"unknown_class"`
	NextOperation string `json:"next_operation"`
}

type Policy struct {
	Identity   string            `json:"identity"`
	Default    string            `json:"default"`
	Precedence []string          `json:"precedence"`
	Rules      map[string]Rule   `json:"rules"`
	CycleRules map[string]string `json:"cycle_rules"`
	Generation GenerationPlan    `json:"generation"`
}

type GenerationPlan struct {
	Language   string `json:"language"`
	Package    string `json:"package"`
	Entrypoint string `json:"entrypoint"`
}

type Edge struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Release string `json:"release"`
	Digest  string `json:"digest"`
	Symbol  string `json:"symbol"`
	Owner   string `json:"owner"`
}

type UnknownEvidence struct {
	Stage         string `json:"stage"`
	Step          string `json:"step"`
	Reason        string `json:"reason"`
	UnknownClass  string `json:"unknown_class"`
	NextOperation string `json:"next_operation"`
	BlockedBy     string `json:"blocked_by"`
}

type Diagnostic struct {
	Code    string           `json:"code"`
	Status  string           `json:"status"`
	Module  string           `json:"module,omitempty"`
	Target  string           `json:"target,omitempty"`
	Symbol  string           `json:"symbol,omitempty"`
	Reason  string           `json:"reason"`
	Unknown *UnknownEvidence `json:"unknown,omitempty"`
}

type LinkedGraph struct {
	Schema          string       `json:"schema"`
	Status          string       `json:"status"`
	Modules         []Module     `json:"modules"`
	Edges           []Edge       `json:"edges"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
	CanonicalDigest string       `json:"canonical_digest"`
}

type Scenario struct {
	Name     string
	Expected string
	Inputs   []string
	Selected bool
}

type Comparison struct {
	Name     string
	Expected string
	Left     []string
	Right    []string
	Selected bool
}

type Conformance struct {
	Identity    string
	Scenarios   []Scenario
	Comparisons []Comparison
}

func (p Policy) RuleFor(code string) (Rule, error) {
	rule, ok := p.Rules[code]
	if !ok {
		return Rule{}, fmt.Errorf("policy %q has no rule %q", p.Identity, code)
	}
	return rule, nil
}

func (r Rule) UnknownEvidence(blockedBy string) *UnknownEvidence {
	if r.Outcome != "UNKNOWN" {
		return nil
	}
	return &UnknownEvidence{
		Stage:         r.Stage,
		Step:          r.Step,
		Reason:        r.Reason,
		UnknownClass:  r.UnknownClass,
		NextOperation: r.NextOperation,
		BlockedBy:     blockedBy,
	}
}
