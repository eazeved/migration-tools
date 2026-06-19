package cmd

import (
	"fmt"
	"os"
	"strings"
)

func safeName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	if s == "" {
		return "unnamed"
	}
	return s
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func ensureEmpty(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("export-dir %q is not empty; choose an empty or new directory", path)
	}
	return nil
}

// matchNameOrID returns true if name or id matches the filter string.
// Comparison is case-insensitive.
func matchNameOrID(name, id, filter string) bool {
	f := strings.ToLower(strings.TrimSpace(filter))
	return strings.ToLower(name) == f || strings.ToLower(id) == f
}
