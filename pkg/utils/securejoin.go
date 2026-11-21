package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SecureJoin joins base and path and returns the requested path, ensuring the
// resolved path stays within base. It evaluates symlinks to prevent escape via
// symlink redirection. Does not return an error if absolute base path does not exist.
func SecureJoin(basePath, path string) (string, error) {
	if !filepath.IsLocal(path) {
		return "", fmt.Errorf("path [%s] is not a local path", path)
	}

	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", err
	}

	resolvedBase, err := resolveExisting(absBase)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Clean(filepath.Join(absBase, path))

	joined, err := resolveExisting(fullPath)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(resolvedBase, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path [%s] is not inside base directory [%s]", path, absBase)
	}

	return filepath.Join(basePath, path), nil
}

func resolveExisting(p string) (string, error) {
	existing := filepath.Clean(p)
	var missing []string
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			break
		}
		missing = append([]string{filepath.Base(existing)}, missing...)
		existing = parent
	}

	resolvedExisting, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", err
	}

	return filepath.Join(append([]string{resolvedExisting}, missing...)...), nil
}
