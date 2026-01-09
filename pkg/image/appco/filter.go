package appco

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func isCorrectAppCoArtifact(a Artifact) bool {
	// must originate from dp.apps.rancher.io
	if !strings.HasPrefix(a.SourceArtifact, "dp.apps.rancher.io/") {
		return false
	}

	// must target registry.suse.com/rancher (Prime/AppCo)
	for _, repo := range a.TargetRepositories {
		if strings.HasPrefix(repo, "registry.suse.com/rancher") {
			return true
		}
	}

	return false
}

func rancherRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	const marker = string(os.PathSeparator) + "rancher"

	idx := strings.LastIndex(wd, marker)
	if idx == -1 {
		return "", fmt.Errorf("appco: unable to locate rancher repo root from %s", wd)
	}

	return wd[:idx+len(marker)], nil
}

func appcoAllowListPath() (string, error) {
	repoRoot, err := rancherRepoRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(
		repoRoot,
		"bin",
		"build",
		"prime-charts",
		"appcoSupportVersions.yaml",
	), nil
}

func loadAppCoAllowList() (map[string]struct{}, error) {
	path, err := appcoAllowListPath()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("appco: failed to open allow list file %s: %w", path, err)
	}
	defer f.Close()

	allowed := make(map[string]struct{})
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		allowed[line] = struct{}{}
	}

	return allowed, scanner.Err()
}
