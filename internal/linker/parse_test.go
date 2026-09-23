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

func TestParsePolicyPreservesHashInsideQuotedReason(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.gooo")
	content := `policy "test" {
default "CLOSED"
precedence "CLOSED"
rule "hash_reason" outcome "CLOSED" stage "stage" step "step" reason "reason #1" # trailing comment
generation "go" package "main" entrypoint "main"
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	policy, err := ParsePolicyFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := policy.Rules["hash_reason"].Reason; got != "reason #1" {
		t.Fatalf("ParsePolicyFile truncated quoted hash: got %q", got)
	}
}

func TestParseConformanceRejectsDuplicateClosingBraces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "conformance.gooo")
	content := `conformance "test" {
scenario "basic" expect "CLOSED" inputs "one" selected "true"
}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseConformanceFile(path); err == nil {
		t.Fatal("ParseConformanceFile accepted duplicate closing braces")
	}
}
