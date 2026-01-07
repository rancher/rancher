package appco

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"

	"github.com/Masterminds/semver/v3"
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

func appcoAllowListURL(rancherVersion string) (string, error) {
	mm, err := majorMinor(rancherVersion)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"https://raw.githubusercontent.com/rancher/prime-charts/dev-v%s/appcoSupportVersions.yaml",
		mm,
	), nil
}

func loadAppCoAllowList(rancherVersion string) (map[string]struct{}, error) {
	url, err := appcoAllowListURL(rancherVersion)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("appco: failed to fetch allow list: %s", resp.Status)
	}

	allowed := make(map[string]struct{})
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		allowed[line] = struct{}{}
	}

	return allowed, scanner.Err()
}

func majorMinor(rancherVersion string) (string, error) {
	v, err := semver.NewVersion(rancherVersion)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d.%d", v.Major(), v.Minor()), nil
}
