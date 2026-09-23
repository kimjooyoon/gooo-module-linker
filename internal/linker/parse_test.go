package linker

import (
	"os"
	"path/filepath"
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

func TestParseConformanceRejectsTrailingClosingBrace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conformance.gooo")
	content := `conformance "test" {
scenario "case" expect "CLOSED" inputs "input.gooo" selected "true"
}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseConformanceFile(path); err == nil {
		t.Fatal("ParseConformanceFile accepted a trailing closing brace")
	}
}
