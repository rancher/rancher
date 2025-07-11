package image

import (
	"fmt"
	"os"
	"strings"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	assertlib "github.com/stretchr/testify/assert"
)

func TestConvertMirroredImages(t *testing.T) {
	testCases := []struct {
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

	assert := assertlib.New(t)
	for _, cs := range testCases {
		imagesSet := cs.inputRawImages
		convertMirroredImages(imagesSet)
		assert.Equal(cs.outputImagesShouldEqual, imagesSet)
	}
}

func TestResolveWithCluster(t *testing.T) {
	if os.Getenv("CATTLE_BASE_REGISTRY") != "" {
		fmt.Println("Skipping TestResolveWithCluster. Can't run the tests with CATTLE_BASE_REGISTRY set")
		return
	}

	type input struct {
		cluster            *v3.Cluster
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
				cluster:            &v3.Cluster{},
			},
			expected: "imagename",
		},
		{
			name: "Cluster empty URL, with default registry",
			input: input{
				image:              "imagename",
				CattleBaseRegistry: "default-registry.com",
				cluster:            &v3.Cluster{},
			},
			expected: "default-registry.com/rancher/imagename",
		},
		{
			name: "Cluster empty URL, with default registry and rancher on image name",
			input: input{
				image:              "rancher/imagename",
				CattleBaseRegistry: "default-registry.com",
				cluster:            &v3.Cluster{},
			},
			expected: "default-registry.com/rancher/imagename",
		},
	}

	if err := settings.SystemDefaultRegistry.Set(""); err != nil {
		t.Errorf("Failed to test TestResolveWithCluster(), unable to set SystemDefaultRegistry with the err: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := settings.SystemDefaultRegistry.Set(tt.input.CattleBaseRegistry); err != nil {
				t.Errorf("Failed to test TestResolveWithCluster(), unable to set SystemDefaultRegistry. err: %v", err)
			}
			assertlib.Equalf(t, tt.expected, ResolveWithCluster(tt.input.image, tt.input.cluster), "ResolveWithCluster(%v, %v)", tt.input.image, tt.input.cluster)
		})
	}

	if err := settings.SystemDefaultRegistry.Set(""); err != nil {
		t.Errorf("Failed to clean up TestResolveWithCluster(), unable to set SystemDefaultRegistry with the err: %v", err)
	}
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
			assertlib.NoError(t, err)

			for _, expected := range tt.expected {
				found := false
				for _, image := range imagesList {
					if strings.Contains(image, expected) {
						found = true
						break
					}
				}
				assertlib.True(t, found)
			}

			for _, notexpected := range tt.notExpected {
				found := false
				for _, image := range imagesList {
					if strings.Contains(image, notexpected) {
						found = true
						break
					}
				}

				assertlib.False(t, found)
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

	if err := settings.SystemDefaultRegistry.Set(""); err != nil {
		t.Errorf("Failed to test TestResolveWithCluster(), unable to clean SystemDefaultRegistry. Err: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := settings.SystemDefaultRegistry.Set(tt.input.CattleBaseRegistry); err != nil {
				t.Errorf("Failed to test TestResolveWithCluster(), unable to set SystemDefaultRegistry. Err: %v", err)
			}
			assertlib.Equalf(t, tt.expected, Resolve(tt.input.image), "Resolve(%v)", tt.input.image)
		})
	}

	if err := settings.SystemDefaultRegistry.Set(""); err != nil {
		t.Errorf("Failed to clean up TestResolve(), unable to clean SystemDefaultRegistry. Err: %v", err)
	}
}
