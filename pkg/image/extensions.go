package image

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type ExtensionsConfig struct {
	Config ExportConfig
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

func (e ExtensionsConfig) FetchExtensionImages(imagesSet map[string]map[string]struct{}) error {
	for _, endpoint := range e.Config.GithubEndpoints {
		// Parse the repository name from the URL
		repoName, err := parseRepoName(endpoint.URL)
		if err != nil {
			return err
		}

		// Fetch latest releases from GitHub API for the current endpoint
		latestReleases, err := getLatestReleases(endpoint)
		if err != nil {
			return err
		}

		// Find the latest non-RC release
		latestReleaseTag := findLatestReleaseTag(latestReleases)

		if latestReleaseTag == "" {
			return fmt.Errorf("no suitable release found for endpoint: %s", endpoint.URL)
		}

		// Construct the image name based on the repository name and latestReleaseTag
		var image string
		if repoName == "rancher/ui-plugin-charts" {
			image = "rancher/ui-plugin-catalog:" + latestReleaseTag
		} else {
			image = repoName + ":" + latestReleaseTag
		}
		addSourceToImage(imagesSet, image, "ui-extension")
	}

	return nil
}

// Helper function to parse the repository name from the GitHub API URL
var parseRepoName = func(url string) (string, error) {
	parts := strings.Split(url, "/")

	if len(parts) < 6 || parts[0] != "https:" || parts[2] != "api.github.com" || parts[3] != "repos" {
		return "", fmt.Errorf("invalid GitHub API URL format: %s", url)
	}

	return parts[4] + "/" + parts[5], nil
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
