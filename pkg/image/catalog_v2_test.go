package image

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	primeValues = "image:\n  repository: rancher/prime-extension\n  tag: v1.0.0\n"
	primeChart  = testChart{"prime-extension", "111.0.0", primeValues, ""}
	fleetChart  = testChart{"fleet", "111.0.1", fleetValues, fleetAnnotation}
)

// writeRepository writes the charts in a directory with the name given, in a temporary directory, and returns it.
func writeRepository(t *testing.T, name string, charts ...testChart) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "build", name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	writeTestCharts(t, dir, charts...)
	return dir
}

func getTestArtifacts(chartsPath string) ([]string, []string, error) {
	return GetArtifacts(chartsPath, "", Linux, "2.16.99", nil, nil, nil, "")
}

func TestIsCatalogPathIsTheCheckoutOfRancherCharts(t *testing.T) {
	for _, path := range []string{"build/charts", "./build/charts", "build/charts/", "bin/build/charts", "/home/runner/rancher/bin/build/charts", "charts"} {
		assert.True(t, isCatalogPath(path), path)
	}
	for _, path := range []string{"", ".", "build/prime-charts", "prime-charts", "build/rancher-charts", "build/charts-small-fork", "build/chartsx"} {
		assert.False(t, isCatalogPath(path), path)
	}
}

func TestGetArtifactsReadsPrimeChartsWithTheOldFunction(t *testing.T) {
	prime := writeRepository(t, "prime-charts", primeChart)

	images, _, err := getTestArtifacts(prime)

	require.NoError(t, err, "it has no helm.sh/images annotation, which the old format never had")
	assert.Contains(t, images, "rancher/prime-extension:v1.0.0")
}

func TestGetArtifactsReadsPrimeChartsAndRancherChartsTogether(t *testing.T) {
	prime := writeRepository(t, "prime-charts", primeChart)
	rancher := writeRepository(t, "charts", fleetChart)

	for name, chartsPath := range map[string]string{"prime first": prime + "," + rancher, "rancher first": rancher + "," + prime} {
		t.Run(name, func(t *testing.T) {
			images, sources, err := getTestArtifacts(chartsPath)

			require.NoError(t, err)
			assert.Contains(t, images, "rancher/prime-extension:v1.0.0")
			assert.Contains(t, images, "rancher/fleet:v0.17.0")
			assert.Contains(t, sources, "rancher/prime-extension:v1.0.0 prime-extension:111.0.0")
		})
	}
}

func TestGetArtifactsStillChecksTheAnnotationsOfRancherCharts(t *testing.T) {
	prime := writeRepository(t, "prime-charts", primeChart)
	rancher := writeRepository(t, "charts", testChart{"fleet", "111.0.1", fleetValues, ""})

	_, _, err := getTestArtifacts(prime + "," + rancher)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fleet:111.0.1")
	assert.NotContains(t, err.Error(), "prime-extension")
}

func TestGetArtifactsStopsForADirectoryThatHasEntriesAndIsNotRancherCharts(t *testing.T) {
	catalog := writeRepository(t, "rancher-catalog", fleetChart)
	require.NoError(t, os.MkdirAll(filepath.Join(catalog, "entries"), 0o755))

	_, _, err := getTestArtifacts(catalog)

	require.Error(t, err)
	assert.Contains(t, err.Error(), catalog)
	assert.Contains(t, err.Error(), "entries")
}

func TestCheckPinnedChartsOnlyLooksAtRancherCharts(t *testing.T) {
	prime := writeRepository(t, "prime-charts", primeChart)
	rancher := writeRepository(t, "charts", fleetChart)

	err := CheckPinnedCharts(prime+","+rancher, "2.16.99", []PinnedChart{{Name: "prime-extension", Version: "9.9.9"}}, true)

	assert.NoError(t, err, "a chart of prime-charts is not one of the pinned charts of Rancher")
}

func TestImagesOfCatalogChartsLeavesOutTheChartsOfOtherDirectories(t *testing.T) {
	prime := writeRepository(t, "prime-charts", primeChart)
	rancher := writeRepository(t, "charts", fleetChart)
	lines := []string{
		"rancher/fleet:v0.17.0 fleet:111.0.1",
		"rancher/prime-extension:v1.0.0 prime-extension:111.0.0",
		"rancher/shared:v1 fleet:111.0.1,prime-extension:111.0.0",
		"rancher/rancher:v2.16.0 rancher",
	}

	images, err := ImagesOfCatalogCharts(lines, prime+","+rancher)

	require.NoError(t, err)
	assert.Equal(t, []ChartImage{
		{Image: "rancher/fleet:v0.17.0", Sources: []string{"fleet:111.0.1"}},
		{Image: "rancher/shared:v1", Sources: []string{"fleet:111.0.1"}},
	}, images)
}
