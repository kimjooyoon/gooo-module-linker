package linker

import (
	"strings"
	"testing"
)

func TestLinkRejectsUndeclaredCyclePolicyWithoutCycle(t *testing.T) {
	policy := Policy{
		Identity:   "policy",
		Default:    "CLOSED",
		Precedence: []string{"REFUTED", "UNKNOWN", "CLOSED"},
		Rules:      map[string]Rule{},
		CycleRules: map[string]string{},
		Generation: GenerationPlan{Language: "go", Package: "main", Entrypoint: "main"},
	}
	module := Module{
		Identity:    "module-a",
		Release:     "v1.0.0",
		Digest:      "sha256:" + strings.Repeat("0", 64),
		CyclePolicy: "not-declared",
	}
	if _, err := Link(policy, []Module{module}); err == nil {
		t.Fatal("Link accepted a module with an undeclared cycle policy")
	}
}

