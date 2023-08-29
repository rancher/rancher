package image

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	assertlib "github.com/stretchr/testify/assert"
)

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

func TestGetLatestReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		releases := []Release{
			{TagName: "1.0.0"},
			{TagName: "1.1.0"},
		}
		_ = json.NewEncoder(rw).Encode(releases)
	}))
	defer server.Close()

	assert := assertlib.New(t)

	endpoint := GithubEndpoint{URL: server.URL}
	releases, err := getLatestReleases(endpoint)
	assert.NoError(err)
	assert.Len(releases, 2)
	assert.Equal("1.0.0", releases[0].TagName)
	assert.Equal("1.1.0", releases[1].TagName)
}

func TestFetchExtensionImages(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		releases := []Release{
			{TagName: "1.0.0"},
			{TagName: "1.1.0"},
		}
		_ = json.NewEncoder(rw).Encode(releases)
	}))
	defer mockServer.Close()

	endpoint := GithubEndpoint{URL: mockServer.URL}
	extensions := Extensions{GithubEndpoint: endpoint}

	imagesSet := map[string]map[string]struct{}{
		"rancher/ui-plugin-catalog:1.1.0": {"ui-extension": {}},
	}

	assert := assertlib.New(t)

	err := extensions.FetchExtensionImages(imagesSet)
	assert.NoError(err)

	imageKey := "rancher/ui-plugin-catalog:1.1.0"
	assert.Contains(imagesSet, imageKey)
	assert.Contains(imagesSet[imageKey], "ui-extension")
}

func TestFetchExtensionImages_NoSuitableRelease(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		releases := []Release{
			{TagName: "1.0.0-rc1"},
			{TagName: "1.1.0-rc1"},
		}
		_ = json.NewEncoder(rw).Encode(releases)
	}))
	defer mockServer.Close()

	endpoint := GithubEndpoint{URL: mockServer.URL}
	extensions := Extensions{GithubEndpoint: endpoint}

	imagesSet := map[string]map[string]struct{}{}

	assert := assertlib.New(t)

	err := extensions.FetchExtensionImages(imagesSet)
	assert.Error(err)
	assert.EqualError(err, "no suitable release found for ui-plugin-charts")
}
