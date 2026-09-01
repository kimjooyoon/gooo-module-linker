package linker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func WriteGraph(path string, graph LinkedGraph) error {
	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal linked IR: %w", err)
	}
	data = append(data, '\n')
	return writeFile(path, data)
}

func WriteBytes(path string, data []byte) error {
	return writeFile(path, data)
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
