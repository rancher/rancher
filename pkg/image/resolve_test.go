package image

import (
	"fmt"
	"os"
	"strings"
	"testing"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertMirroredImages(t *testing.T) {
	tests := []struct {
		inputRawImages          map[string]map[string]struct{}
		outputImagesShouldEqual map[string]map[string]struct{}
		caseName                string
	}{
		{
			caseName: "normalize images",
			inputRawImages: map[string]map[string]struct{}{
				"rancher/rke-tools:v0.1.48": {"system": struct{}{}},
				"rancher/rke-tools:v0.1.49": {"system": struct{}{}},
				// for mirror
				"prom/prometheus:v2.0.1":                           {"system": struct{}{}},
				"quay.io/coreos/flannel:v1.2.3":                    {"system": struct{}{}},
				"gcr.io/google_containers/k8s-dns-kube-dns:1.15.0": {"system": struct{}{}},
				"test.io/test:v0.0.1":                              {"test": struct{}{}}, // not in mirror list
			},
			outputImagesShouldEqual: map[string]map[string]struct{}{
				"rancher/coreos-flannel:v1.2.3":   {"system": struct{}{}},
				"rancher/k8s-dns-kube-dns:1.15.0": {"system": struct{}{}},
				"rancher/prom-prometheus:v2.0.1":  {"system": struct{}{}},
				"rancher/rke-tools:v0.1.48":       {"system": struct{}{}},
				"rancher/rke-tools:v0.1.49":       {"system": struct{}{}},
				"test.io/test:v0.0.1":             {"test": struct{}{}},
			},
		},
	}

	for _, cs := range tests {
		imagesSet := cs.inputRawImages
		convertMirroredImages(imagesSet)
		assert.Equal(t, cs.outputImagesShouldEqual, imagesSet)
	}
}

func TestResolveWithCluster(t *testing.T) {
	if os.Getenv("CATTLE_BASE_REGISTRY") != "" {
		fmt.Println("Skipping TestResolveWithCluster. Can't run the tests with CATTLE_BASE_REGISTRY set")
		return
	}

	type input struct {
		cluster            *apisv3.Cluster
		image              string
		CattleBaseRegistry string
	}
	tests := []struct {
		name     string
		input    input
		expected string
	}{
		{
			name: "No cluster no default registry",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "",
				cluster:            nil,
			},
			expected: "imagename",
		},
		{
			name: "No cluster with default registry, image without rancher/",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "custom-registry",
				cluster:            nil,
			},
			expected: "custom-registry/rancher/imagename",
		},
		{
			name: "No cluster with default registry, image with rancher/",
			input: input{
				image:              "rancher/imagename",
				CattleBaseRegistry: "custom-registry",
				cluster:            nil,
			},
			expected: "custom-registry/rancher/imagename",
		},
		{
			name: "Cluster empty URL, no default registry",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "",
				cluster:            &apisv3.Cluster{},
			},
			expected: "imagename",
		},
		{
			name: "Cluster empty URL, with default registry",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "default-registry.com",
				cluster:            &apisv3.Cluster{},
			},
			expected: "default-registry.com/rancher/imagename",
		},
		{
			name: "Cluster empty URL, with default registry and rancher on image name",
			input: input{
				image:              "rancher/imagename",
				CattleBaseRegistry: "default-registry.com",
				cluster:            &apisv3.Cluster{},
			},
			expected: "default-registry.com/rancher/imagename",
		},
	}

	err := settings.SystemDefaultRegistry.Set("")
	require.NoError(t, err, "Failed to test TestResolveWithCluster(), unable to set SystemDefaultRegistry")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := settings.SystemDefaultRegistry.Set(tt.input.CattleBaseRegistry)
			require.NoError(t, err, "Failed to test TestResolveWithCluster(), unable to set SystemDefaultRegistry")
			assert.Equalf(t, tt.expected, ResolveWithCluster(tt.input.image, tt.input.cluster), "ResolveWithCluster(%v, %v)", tt.input.image, tt.input.cluster)
		})
	}

	err = settings.SystemDefaultRegistry.Set("")
	require.NoError(t, err, "Failed to test TestResolveWithCluster(), unable to set SystemDefaultRegistry")
}

func TestGetImages(t *testing.T) {
	// Setup a httpserver
	server := setupTestServer()
	defer server.Close()

	// Mock the parseRepoName function to return the expected repoName
	originalParseRepoName := parseRepoName
	parseRepoName = func(url string) (string, error) {
		return "rancher/ui-plugin-catalog", nil
	}
	defer func() {
		parseRepoName = originalParseRepoName
	}()

	tests := []struct {
		name         string
		expected     []string
		notExpected  []string
		exportConfig ExportConfig
	}{
		{
			name: "exportConfig is completely empty",
			exportConfig: ExportConfig{
				ChartsPath:      "",
				OsType:          Linux,
				RancherVersion:  "",
				GithubEndpoints: []GithubEndpoint{},
			},
			expected:    []string{},
			notExpected: []string{"rancher/ui-plugin-catalog"},
		},
		{
			name: "only extensions is set in exportConfig",
			exportConfig: ExportConfig{
				ChartsPath:     "",
				OsType:         Linux,
				RancherVersion: "",
				GithubEndpoints: []GithubEndpoint{
					{URL: server.URL},
				},
			},
			expected:    []string{"rancher/ui-plugin-catalog"},
			notExpected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imagesList, _, err := GetImages(tt.exportConfig, make(map[string][]string), []string{})
			assert.NoError(t, err)

			for _, expected := range tt.expected {
				found := false
				for _, image := range imagesList {
					if strings.Contains(image, expected) {
						found = true
						break
					}
				}
				assert.True(t, found)
			}

			for _, notexpected := range tt.notExpected {
				found := false
				for _, image := range imagesList {
					if strings.Contains(image, notexpected) {
						found = true
						break
					}
				}

				assert.False(t, found)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	if os.Getenv("CATTLE_BASE_REGISTRY") != "" {
		fmt.Println("Skipping TestResolve. Can't run the tests with CATTLE_BASE_REGISTRY set")
		return
	}

	type input struct {
		image              string
		CattleBaseRegistry string
	}
	tests := []struct {
		name     string
		input    input
		expected string
	}{
		{
			name: "No default",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "",
			},
			expected: "imagename",
		},
		{
			name: "Default without rancher",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "default-registry.com",
			},
			expected: "default-registry.com/rancher/imagename",
		},
		{
			name: "Default with rancher",
			input: input{
				image:              "rancher/imagename",
				CattleBaseRegistry: "default-registry.com",
			},
			expected: "default-registry.com/rancher/imagename",
		},
	}

	err := settings.SystemDefaultRegistry.Set("")
	require.NoError(t, err, "Failed to test TestResolveWithCluster(), unable to clean SystemDefaultRegistry")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := settings.SystemDefaultRegistry.Set(tt.input.CattleBaseRegistry)
			require.NoError(t, err, "Failed to test TestResolveWithCluster(), unable to set SystemDefaultRegistry")
			assert.Equalf(t, tt.expected, Resolve(tt.input.image), "Resolve(%v)", tt.input.image)
		})
	}

	err = settings.SystemDefaultRegistry.Set("")
	require.NoError(t, err, "Failed to clean up TestResolve(), unable to clean SystemDefaultRegistry")

}

func TestSetRequiredImages(t *testing.T) {
	t.Run("linux images must contain kube-api-auth", func(t *testing.T) {
		imagesSet := make(map[string]map[string]struct{})
		setRequiredImages(Linux, imagesSet)

		kubeApiAuth := apisv3.ToolsSystemImages.AuthSystemImages.KubeAPIAuth
		require.Contains(t, imagesSet, kubeApiAuth)
		assert.Contains(t, imagesSet[kubeApiAuth], imageSourceSystem)
	})
	t.Run("windows images should be empty", func(t *testing.T) {
		imagesSet := make(map[string]map[string]struct{})
		setRequiredImages(Windows, imagesSet)

		require.Empty(t, imagesSet)
	})
}
