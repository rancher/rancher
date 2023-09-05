package image

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	assertlib "github.com/stretchr/testify/assert"
)

func setupTestServer() *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		releases := []Release{
			{TagName: "1.0.0"},
			{TagName: "1.1.0"},
			{TagName: "1.1.0-rc1"},
			{TagName: "2.0.0-rc2"},
			{TagName: "1.1.1"},
			{TagName: "2.0.0"},
		}
		_ = json.NewEncoder(rw).Encode(releases)
	}))

	return server
}

func TestParseRepoName(t *testing.T) {
	assert := assertlib.New(t)

	// Test with a valid GitHub API URL
	validURL := "https://api.github.com/repos/some-user/some-repo/releases"
	repoName, err := parseRepoName(validURL)
	assert.NoError(err)
	assert.Equal("some-user/some-repo", repoName)

	// Test with an invalid GitHub API URL
	invalidURL := "https://example.com/invalid/url"
	_, err = parseRepoName(invalidURL)
	assert.Error(err)
	assert.Contains(err.Error(), "invalid GitHub API URL format")
}

func TestGetLatestReleases(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	assert := assertlib.New(t)

	endpoint := GithubEndpoint{URL: server.URL}
	releases, err := getLatestReleases(endpoint)
	assert.NoError(err)
	assert.Len(releases, 6)
	assert.Equal("1.0.0", releases[0].TagName)
	assert.Equal("1.1.0", releases[1].TagName)
	assert.Equal("1.1.0-rc1", releases[2].TagName)
	assert.Equal("2.0.0-rc2", releases[3].TagName)
	assert.Equal("1.1.1", releases[4].TagName)
	assert.Equal("2.0.0", releases[5].TagName)
}

func TestFindLatestReleaseTag(t *testing.T) {
	releases := []Release{
		{TagName: "1.0.0"},
		{TagName: "1.2.0-rc1"},
		{TagName: "1.1.0"},
		{TagName: "2.0.0-rc2"},
	}

	assert := assertlib.New(t)

	tag := findLatestReleaseTag(releases)
	assert.Equal("1.1.0", tag)
}

func TestFetchExtensionImages(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	endpoints := []GithubEndpoint{{URL: server.URL}}
	extensions := ExtensionsConfig{GithubEndpoints: endpoints}

	imagesSet := map[string]map[string]struct{}{}

	// Mock the parseRepoName function to return the expected repoName
	originalParseRepoName := parseRepoName
	parseRepoName = func(url string) (string, error) {
		return "some-org/some-repo", nil
	}
	defer func() {
		parseRepoName = originalParseRepoName
	}()

	assert := assertlib.New(t)

	err := extensions.FetchExtensionImages(imagesSet)
	assert.NoError(err)

	imageKey := "some-org/some-repo:2.0.0"
	assert.Contains(imagesSet, imageKey)
	assert.Contains(imagesSet[imageKey], "ui-extension")
}

func TestFetchExtensionImages_NoSuitableRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		releases := []Release{
			{TagName: "1.0.0-rc1"},
			{TagName: "1.1.0-rc1"},
		}
		_ = json.NewEncoder(rw).Encode(releases)
	}))
	defer server.Close()

	endpoints := []GithubEndpoint{{URL: server.URL}}
	extensions := ExtensionsConfig{GithubEndpoints: endpoints}

	imagesSet := map[string]map[string]struct{}{}

	originalParseRepoName := parseRepoName
	parseRepoName = func(url string) (string, error) {
		return "some-org/some-repo", nil
	}
	defer func() {
		parseRepoName = originalParseRepoName
	}()

	assert := assertlib.New(t)

	err := extensions.FetchExtensionImages(imagesSet)
	assert.Error(err)
	assert.EqualError(err, "no suitable release found for endpoint: "+server.URL)
}
