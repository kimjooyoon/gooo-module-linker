package linker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePolicyRejectsDuplicateRuleIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.gooo")
	content := `policy "test" {
default "CLOSED"
precedence "CLOSED"
rule "duplicate" outcome "CLOSED" stage "stage" step "step" reason "reason"
rule "duplicate" outcome "CLOSED" stage "stage" step "step" reason "replacement"
generation "go" package "main" entrypoint "main"
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePolicyFile(path); err == nil {
		t.Fatal("ParsePolicyFile accepted duplicate rule IDs")
	}
}

func TestParsePolicyRejectsDuplicateSingletonDeclarations(t *testing.T) {
	base := `policy "test" {
default "CLOSED"
precedence "REFUTED" "UNKNOWN" "CLOSED"
rule "cycle_acyclic" outcome "REFUTED" stage "cycle" step "policy" reason "reason"
cycle_policy "acyclic" rule "cycle_acyclic"
generation "go" package "main" entrypoint "main"
}`
	for _, test := range []struct {
		name   string
		anchor string
	}{
		{name: "default", anchor: `default "CLOSED"`},
		{name: "precedence", anchor: `precedence "REFUTED" "UNKNOWN" "CLOSED"`},
		{name: "cycle policy", anchor: `cycle_policy "acyclic" rule "cycle_acyclic"`},
		{name: "generation", anchor: `generation "go" package "main" entrypoint "main"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			content := strings.Replace(base, test.anchor, test.anchor+"\n"+test.anchor, 1)
			path := filepath.Join(t.TempDir(), "policy.gooo")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ParsePolicyFile(path); err == nil {
				t.Fatalf("ParsePolicyFile accepted duplicate %s declaration", test.name)
			}
		})
	}
}
