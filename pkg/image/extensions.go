package image

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type Extensions struct {
	Config         ExportConfig
	GithubEndpoint GithubEndpoint
}

type Release struct {
	TagName string `json:"tag_name"`
}

type GithubEndpoint struct {
	URL string
}

var ExtensionEndpoints = []GithubEndpoint{
	{URL: "https://api.github.com/repos/rancher/ui-plugin-charts/releases"},
}

func ExtensionConfig(endpointIndex int) Extensions {
	if endpointIndex >= len(ExtensionEndpoints) {
		endpointIndex = 0
	}

	return Extensions{
		Config:         ExportConfig{},
		GithubEndpoint: ExtensionEndpoints[endpointIndex],
	}
}

func (e Extensions) FetchExtensionImages(imagesSet map[string]map[string]struct{}) error {
	// Fetch latest releases from GitHub API
	latestReleases, err := getLatestReleases(e.GithubEndpoint)
	if err != nil {
		return err
	}

	// Find the latest non-RC release
	latestReleaseTag := findLatestReleaseTag(latestReleases)

	if latestReleaseTag == "" {
		return fmt.Errorf("no suitable release found for ui-plugin-charts")
	}

	image := "rancher/ui-plugin-catalog:" + latestReleaseTag
	addSourceToImage(imagesSet, image, "ui-extension")

	return nil
}

func getLatestReleases(githubURL GithubEndpoint) ([]Release, error) {
	// Get the releases from GitHub API
	resp, err := http.Get(githubURL.URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var releases []Release
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&releases); err != nil {
		return nil, err
	}

	return releases, nil
}

func findLatestReleaseTag(releases []Release) string {
	var latestReleaseTag string
	latestVersion := semver.MustParse("0.0.0")

	for _, release := range releases {
		// Exclude release candidates (e.g., "1.0.0-rc1")
		if !strings.Contains(release.TagName, "-rc") {
			version, err := semver.NewVersion(release.TagName)

			if err == nil && version.GreaterThan(latestVersion) {
				latestVersion = version
				latestReleaseTag = release.TagName
			}
		}
	}

	return latestReleaseTag
}
