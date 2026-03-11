package module

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindModuleRoot walks up from start looking for a go.mod file and returns the
// directory containing it. The start path is resolved to an absolute path first.
func FindModuleRoot(start string) (string, error) {
	d, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("could not find go.mod above %s", start)
		}
		d = parent
	}
}

// FindModuleInfo returns the module path and root directory by walking up from
// the current working directory. Returns empty strings if no go.mod is found.
func FindModuleInfo() (modPath string, modRoot string) {
	dir, err := os.Getwd()
	if err != nil {
		return "", ""
	}
	root, err := FindModuleRoot(dir)
	if err != nil {
		return "", ""
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), root
		}
	}
	return "", ""
}
