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

func TestParseModulePreservesHashesInsideQuotedFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "module.gooo")
	content := `module "module#1" release "v1" digest "sha256#1" cycle_policy "reject#cycles" {
export "symbol#1" owner "owner#1"
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	module, err := ParseModuleFile(path)
	if err != nil {
		t.Fatalf("ParseModuleFile rejected hashes inside quoted fields: %v", err)
	}
	if module.Identity != "module#1" || module.Digest != "sha256#1" || module.CyclePolicy != "reject#cycles" {
		t.Fatalf("module quoted fields lost hashes: %+v", module)
	}
	if len(module.Exports) != 1 || module.Exports[0].Symbol != "symbol#1" || module.Exports[0].Owner != "owner#1" {
		t.Fatalf("export quoted fields lost hashes: %+v", module.Exports)
	}
}
