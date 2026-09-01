package linker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

func ModuleDigest(module Module) string {
	canonical := moduleCanonical(module)
	digest := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func moduleCanonical(module Module) string {
	exports := append([]Export(nil), module.Exports...)
	imports := append([]Import(nil), module.Imports...)
	sort.Slice(exports, func(i, j int) bool {
		if exports[i].Symbol != exports[j].Symbol {
			return exports[i].Symbol < exports[j].Symbol
		}
		return exports[i].Owner < exports[j].Owner
	})
	sort.Slice(imports, func(i, j int) bool {
		left := imports[i].Module + "\x00" + imports[i].Release + "\x00" + imports[i].Symbol + "\x00" + imports[i].Owner
		right := imports[j].Module + "\x00" + imports[j].Release + "\x00" + imports[j].Symbol + "\x00" + imports[j].Owner
		return left < right
	})
	result := "module:" + module.Identity + "\nrelease:" + module.Release + "\ncycle_policy:" + module.CyclePolicy + "\n"
	for _, export := range exports {
		result += "export:" + export.Symbol + "|owner:" + export.Owner + "\n"
	}
	for _, imp := range imports {
		result += "import:" + imp.Module + "|release:" + imp.Release + "|symbol:" + imp.Symbol + "|owner:" + imp.Owner + "\n"
	}
	return result
}

func CanonicalGraphDigest(graph LinkedGraph) (string, error) {
	copyGraph := graph
	copyGraph.CanonicalDigest = ""
	encoded, err := json.Marshal(copyGraph)
	if err != nil {
		return "", fmt.Errorf("marshal canonical graph: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func sortModule(module *Module) {
	sort.Slice(module.Exports, func(i, j int) bool {
		if module.Exports[i].Symbol != module.Exports[j].Symbol {
			return module.Exports[i].Symbol < module.Exports[j].Symbol
		}
		return module.Exports[i].Owner < module.Exports[j].Owner
	})
	sort.Slice(module.Imports, func(i, j int) bool {
		left := module.Imports[i].Module + "\x00" + module.Imports[i].Release + "\x00" + module.Imports[i].Digest + "\x00" + module.Imports[i].Symbol + "\x00" + module.Imports[i].Owner
		right := module.Imports[j].Module + "\x00" + module.Imports[j].Release + "\x00" + module.Imports[j].Digest + "\x00" + module.Imports[j].Symbol + "\x00" + module.Imports[j].Owner
		return left < right
	})
}
