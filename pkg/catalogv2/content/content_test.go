package content

import (
	"reflect"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
)

func TestFilterReleasesSemver(t *testing.T) {
	tests := []struct {
		testName               string
		chartVersionAnnotation string
		rancherVersion         string
		expectedPass           bool
	}{
		{
			"rancher version in range comparison with `>= <`style comparison",
			">= 2.5.0-alpha3 <2.6.0",
			"v2.5.0+123",
			true,
		},
		{
			"rancher version in range comparison with `> <`style comparison",
			">2.5.0 <2.6.0",
			"v2.5.7",
			true,
		},
		{
			"rancher version in range comparison with `> <=`style comparison",
			">2.5.0-rc1 <=2.6.0-0",
			"v2.5.0-rc2", //SemVer comparisons using constraints with prerelease will be evaluated using an ASCII sort order, per the spec
			true,
		},
		{
			"rancher version in range comparison with `>= <=`style comparison",
			">=2.5.0-alpha2 <=2.6.0",
			"v2.5.0", //Pre-release versions would be skipped with this comparison
			true,
		},
		{
			"rancher version in range comparison with `~` style comparison",
			"~2.5.x", //equivalent to ">= 2.5.0 < 2.6.0"
			"v2.5.7",
			true,
		},
		{
			"rancher version in range comparison with `<` style comparison",
			"<2.6.001",
			"v2.6.000",
			true,
		},
		{
			"rancher version in range comparison with `<=` style comparison",
			"<=2.5.8-rc7",
			"v2.5.8-rc2+123", //SemVer comparisons using constraints with prerelease will be evaluated using an ASCII sort order, per the spec
			true,
		},
		{
			"rancher version in range comparison with `>=` style comparison",
			">= 2.4.3-r8",
			"v2.4.3-r9",
			true,
		},
		{
			"rancher version in range comparison with `>` style comparison",
			">2.4.3",
			"v2.4.4",
			true,
		},
		{
			"rancher version in range comparison with `-` style comparison",
			"2.5 - 2.6.3", //equivalent to ">= 2.5 <= 2.6.3"
			"v2.5.9",
			true,
		},
		{
			"rancher version in range comparison with `^` style comparison",
			"^2.7.5", //equivalent to ">= 2.7.5, < 2.8.0"
			"v2.7.8",
			true,
		},
		{
			"rancher version out of range comparison with `>= <`style comparison",
			">= 2.5.0-alpha3 <2.6.0-0",
			"v2.5.0-alpha2", //SemVer comparisons using constraints with prerelease will be evaluated using an ASCII sort order, per the spec
			false,
		},
		{
			"rancher prerelease version in range comparison with `>= <`style comparison",
			">= 2.5.0-alpha3 <2.6.0-0",
			"v2.5.0-alpha4", //SemVer comparisons using constraints with prerelease will be evaluated using an ASCII sort order, per the spec
			true,
		},
		{
			"rancher version out of range comparison with `> <`style comparison",
			">2.5.0 <2.6.0",
			"v2.5.3-alpha3",
			true,
		},
		{
			"rancher version out of range comparison with `> <=`style comparison",
			"> 2.5.0-alpha <=2.6.0",
			"v2.5.1-alpha",
			true,
		},
		{
			"rancher version out of range comparison with `>= <=`style comparison",
			">=2.5.0-rc1 <=2.6.0",
			"v2.4.2", //Pre-release versions would be skipped with this comparison
			false,
		},
		{
			"rancher version out of range comparison with `~` style comparison",
			"~2.5.040", //equivalent to >= 2.5.0, < 2.6.0
			"v2.5.039",
			false,
		},
		{
			"rancher version out of range comparison with `<` style comparison",
			"<2.6.0-alpha",
			"v2.7.3",
			false,
		},
		{
			"rancher version out of range comparison with `<=` style comparison",
			"<=2.6.0",
			"v2.6.1",
			false,
		},
		{
			"rancher version out of range comparison with `>=` style comparison",
			">= 2.4.3",
			"v2.4.2-alpha1",
			false,
		},
		{
			"rancher version out of range comparison with `>` style comparison",
			">2.4.3",
			"v2.4.3",
			false,
		},
		{
			"rancher version out range comparison with `-` style comparison",
			"2.5 - 2.6.3", //equivalent to ">= 2.5 <= 2.6.3"
			"v2.4.9",
			false,
		},
		{
			"rancher version out range comparison with `^` style comparison",
			"^2.7.x", //equivalent to ">= 2.7.0 < 3.0.0"
			"v3.0.0",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			indexFile := repo.IndexFile{
				Entries: map[string]repo.ChartVersions{
					"test-chart": {
						{
							Metadata: &chart.Metadata{
								Name:    "test-chart",
								Version: "1.0.0",
								Annotations: map[string]string{
									"catalog.cattle.io/rancher-version": tt.chartVersionAnnotation,
								},
							},
							URLs:    nil,
							Created: time.Time{},
							Removed: false,
							Digest:  "",
						},
					},
				},
			}
			filteredIndexFile := repo.IndexFile{
				Entries: map[string]repo.ChartVersions{
					"test-chart": {
						{
							Metadata: &chart.Metadata{
								Name:    "test-chart",
								Version: "1.0.0",
								Annotations: map[string]string{
									"catalog.cattle.io/rancher-version": tt.chartVersionAnnotation,
								},
							},
							URLs:    nil,
							Created: time.Time{},
							Removed: false,
							Digest:  "",
						},
					},
				},
			}
			contentManager := Manager{}
			settings.ServerVersion.Set(tt.rancherVersion)
			contentManager.filterReleases(&filteredIndexFile, nil, false)
			result := reflect.DeepEqual(indexFile, filteredIndexFile)
			assert.Equal(t, tt.expectedPass, result)
			if result != tt.expectedPass {
				t.Logf("Expected %v, got %v for %s with rancher version %s", tt.expectedPass, result, tt.chartVersionAnnotation, tt.rancherVersion)
			}
		})
	}
}

func TestFilteringReleases(t *testing.T) {
	tests := []struct {
		testName                    string
		chartVersionAnnotation      string
		rancherVersion              string
		kubernetesVersionAnnotation string
		kubernetesVersion           string
		skipFiltering               bool
		expectedPass                bool
	}{
		{
			"Index with chart that has no filters and skips filtering",
			"",
			"",
			"",
			"",
			true,
			true,
		},
		{
			"Index with chart that has rancher-version annotation filter and skips filtering",
			">= 2.5.0-alpha3 <2.6.0",
			"v2.5.0+123",
			"",
			"v1.21.0",
			true,
			true,
		},
		{
			"Index with chart that has kubernetes version filter and skips filtering",
			"",
			"v2.5.7",
			"v1.20.0",
			"v1.21.0",
			true,
			true,
		},
		{
			"Index with chart that has kubernetes version and rancher-version annotation filter and skips filtering",
			">2.5.0-rc1 <=2.6.0-0",
			"v2.5.0-rc2",
			"v1.20.0",
			"v1.21.0",
			true,
			true,
		},
		{
			"Index with chart that has no filters and applies filters",
			"",
			"",
			"",
			"",
			false,
			true,
		},
		{
			"Index with chart that has rancher-version annotation filter and applies filtering",
			">= 2.5.0-alpha3 <2.6.0",
			"v2.5.0+123",
			"",
			"v1.21.0",
			false,
			true,
		},
		{
			"Index with chart that has kubernetes version filter and applies filtering",
			"",
			"v2.5.7",
			"v1.20.0",
			"v1.21.0",
			false,
			false,
		},
		{
			"Index with chart that has kubernetes version and rancher-version annotation filter and applies filtering",
			">2.5.0-rc1 <=2.6.0-0",
			"v2.5.0-rc2",
			"v1.20.0",
			"v1.21.0",
			false,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			indexFile := repo.IndexFile{
				Entries: map[string]repo.ChartVersions{
					"test-chart": {
						{
							Metadata: &chart.Metadata{
								Name:    "test-chart",
								Version: "1.0.0",
								Annotations: map[string]string{
									"catalog.cattle.io/rancher-version": tt.chartVersionAnnotation,
								},
								KubeVersion: tt.kubernetesVersionAnnotation,
							},
							URLs:    nil,
							Created: time.Time{},
							Removed: false,
							Digest:  "",
						},
					},
				},
			}
			filteredIndexFile := repo.IndexFile{
				Entries: map[string]repo.ChartVersions{
					"test-chart": {
						{
							Metadata: &chart.Metadata{
								Name:    "test-chart",
								Version: "1.0.0",
								Annotations: map[string]string{
									"catalog.cattle.io/rancher-version": tt.chartVersionAnnotation,
								},
								KubeVersion: tt.kubernetesVersionAnnotation,
							},
							URLs:    nil,
							Created: time.Time{},
							Removed: false,
							Digest:  "",
						},
					},
				},
			}
			contentManager := Manager{}
			settings.ServerVersion.Set(tt.rancherVersion)
			kubeVersion, _ := semver.NewVersion(tt.kubernetesVersion)
			contentManager.filterReleases(&filteredIndexFile, kubeVersion, tt.skipFiltering)
			result := reflect.DeepEqual(indexFile, filteredIndexFile)
			assert.Equal(t, tt.expectedPass, result)
			if result != tt.expectedPass {
				t.Logf("Expected %v, got %v for %s with rancher version %s", tt.expectedPass, result, tt.chartVersionAnnotation, tt.rancherVersion)
			}
		})
	}
}
func TestFilteringReleaseKubeVersionAnnotation(t *testing.T) {
	tests := []struct {
		testName                    string
		chartVersionAnnotation      string
		rancherVersion              string
		ChartKubeVersionAnnotation string
		kubernetesVersion           string
		skipFiltering               bool
		expectedPass                bool
	}{
		{
			"Index with chart that has no filters and skips filtering",
			"",
			"",
			"",
			"",
			true,
			true,
		},
		{
			"Index with chart that has kube-version annotation filter and skips filtering",
			"",
			"v2.5.0+123",
			"1.18 - 1.20",
			"v1.21.0",
			true,
			true,
		},
		{
			"Index with chart that has kube-version annotation filter - case 1",
			"",
			"v2.5.0+123",
			"1.18 - 1.20",
			"v1.21.0",
			false,
			false,
		},
		{
			"Index with chart that has kube-version annotation filter - case 2",
			"",
			"v2.5.0+123",
			"1.18 - 1.21",
			"v1.21.0",
			false,
			true,
		},
		{
			"Index with chart that has kube-version annotation filter - case 3",
			"",
			"v2.5.0+123",
			" = 1.20",
			"v1.21.0",
			false,
			false,
		},
		{
			"Index with chart that has kube-version annotation filter - case 4",
			"",
			"v2.5.0+123",
			" = 1.21.1",
			"v1.21.0",
			false,
			false,
		},
		{
			"Index with chart that has kube-version annotation filter - case 5",
			"",
			"v2.5.0+123",
			" >= 1.21",
			"v1.21.0",
			false,
			true,
		},
		{
			"Index with chart that has kube-version annotation filter - case 6",
			"",
			"v2.5.0+123",
			" <= 1.22",
			"v1.21.0",
			false,
			true,
		},
		{
			"Index with chart that has kube-version annotation filter - case 7",
			"",
			"v2.5.0+123",
			" < 1.22.0-0",
			"v1.21.0",
			false,
			true,
		},
		{
			"Index with chart that has kube-version annotation filter - case 7",
			"",
			"v2.5.0+123",
			">= 1.19, <= 1.21",
			"v1.21.0",
			false,
			true,
		},
		{
			"Index with chart that has kube-version annotation filter - case 8",
			"",
			"v2.5.0+123",
			" >= 1.19, <= 1.20",
			"v1.21.0",
			false,
			false,
		},
		{
			"Index with chart that has kube-version annotation filter - case 9",
			"",
			"v2.5.0+123",
			" >= 1.19.0-0 < 1.22.0",
			"v1.21.0",
			false,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			indexFile := repo.IndexFile{
				Entries: map[string]repo.ChartVersions{
					"test-chart": {
						{
							Metadata: &chart.Metadata{
								Name:    "test-chart",
								Version: "1.0.0",
								Annotations: map[string]string{
									"catalog.cattle.io/rancher-version": tt.chartVersionAnnotation,
									"catalog.cattle.io/kube-version": tt.ChartKubeVersionAnnotation,
								},
							},
							URLs:    nil,
							Created: time.Time{},
							Removed: false,
							Digest:  "",
						},
					},
				},
			}
			filteredIndexFile := repo.IndexFile{
				Entries: map[string]repo.ChartVersions{
					"test-chart": {
						{
							Metadata: &chart.Metadata{
								Name:    "test-chart",
								Version: "1.0.0",
								Annotations: map[string]string{
									"catalog.cattle.io/rancher-version": tt.chartVersionAnnotation,
									"catalog.cattle.io/kube-version": tt.ChartKubeVersionAnnotation,
								},
							},
							URLs:    nil,
							Created: time.Time{},
							Removed: false,
							Digest:  "",
						},
					},
				},
			}
			contentManager := Manager{}
			settings.ServerVersion.Set(tt.rancherVersion)
			kubeVersion, _ := semver.NewVersion(tt.kubernetesVersion)
			contentManager.filterReleases(&filteredIndexFile, kubeVersion, tt.skipFiltering)
			result := reflect.DeepEqual(indexFile, filteredIndexFile)
			assert.Equal(t, tt.expectedPass, result)
			if result != tt.expectedPass {
				t.Logf("Expected %v, got %v for %s with chart kubeVersion annotation %s", tt.expectedPass, result, kubeVersion, tt.ChartKubeVersionAnnotation)
			}
		})
	}
}
