package content

import (
	"reflect"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
)

func TestFilterReleases(t *testing.T) {
	tests := []struct {
		testName               string
		chartVersionAnnotation string
		rancherVersion         string
		expectedPass           bool
	}{
		{
			"rancher version in range comparison with `>= <`style comparison",
			">= 2.5.0-alpha3 <2.6.0",
			"v2.5.0+123", //SemVer comparisons using constraints without a prerelease comparator will skip prerelease versions
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
			"rancher version out of range comparison with `> <`style comparison",
			">2.5.0 <2.6.0",
			"v2.5.3-alpha3", //SemVer comparisons using constraints without a prerelease comparator will skip prerelease versions
			false,
		},
		{
			"rancher version out of range comparison with `> <=`style comparison",
			"> 2.5.0-alpha <=2.6.0",
			"v2.5.1-alpha", //SemVer comparisons using constraints without a prerelease comparator will skip prerelease versions
			false,
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
			"v2.4.2-alpha1", //SemVer comparisons using constraints without a prerelease comparator will skip prerelease versions
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
			contentManager.filterReleases(&filteredIndexFile, nil)
			result := reflect.DeepEqual(indexFile, filteredIndexFile)
			assert.Equal(t, tt.expectedPass, result)
			if result != tt.expectedPass {
				t.Logf("Expected %v, got %v for %s with rancher version %s", tt.expectedPass, result, tt.chartVersionAnnotation, tt.rancherVersion)
			}
		})
	}
}
