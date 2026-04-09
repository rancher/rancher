package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SecureJoin joins base and path and returns the absolute path, ensuring the
// result stays within base. It evaluates symlinks to prevent escape via
// symlink redirection. Does not return an error if absolute base path does not exist.
func SecureJoin(basePath, path string) (string, error) {
	if !filepath.IsLocal(path) {
		return "", fmt.Errorf("path [%s] is not a local path", path)
	}

	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absBase); err == nil {
		absBase = resolved
	}

	fullPath := filepath.Join(absBase, path)
	joined, err := filepath.EvalSymlinks(filepath.Join(absBase, path))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		joined = filepath.Clean(fullPath)
	}

	rel, err := filepath.Rel(absBase, joined)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path [%s] is not inside base directory [%s]", path, absBase)
	}

	return filepath.Join(basePath, path), nil
}
