package image

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testChart is a chart version of an index that supports Rancher 2.16, whose archive is in assets/.
type testChart struct {
	name    string
	version string
	// values is the values.yaml of the archive.
	values string
	// annotation is the value of its helm.sh/images annotation, none when it is empty.
	annotation string
}

func (c testChart) source() string {
	return c.name + ":" + c.version
}

// writeTestCharts writes an index.yaml with the charts, and the archive of each in assets/.
func writeTestCharts(t *testing.T, root string, charts ...testChart) {
	t.Helper()
	index := "apiVersion: v1\nentries:\n"
	for _, c := range charts {
		index += fmt.Sprintf("  %s:\n  - annotations:\n      catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'\n", c.name)
		if c.annotation != "" {
			index += "      helm.sh/images: |\n"
			for _, line := range strings.Split(strings.TrimRight(c.annotation, "\n"), "\n") {
				index += "        " + line + "\n"
			}
		}
		archive := fmt.Sprintf("assets/%[1]s/%[1]s-%[2]s.tgz", c.name, c.version)
		index += fmt.Sprintf("    apiVersion: v2\n    name: %s\n    version: %s\n    urls:\n    - %s\n", c.name, c.version, archive)
		writeTestArchive(t, root, archive, c.values)
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.yaml"), []byte(index), 0o644))
}

func fetchTestImagesOf(t *testing.T, root string, osType OSType) (map[string]map[string]struct{}, error) {
	t.Helper()
	imagesSet := map[string]map[string]struct{}{}
	charts := Charts{ExportConfig{ChartsPath: root, RancherVersion: "2.16.99", OsType: osType}}
	err := charts.FetchImagesV2(imagesSet)
	return imagesSet, err
}

const (
	fleetValues     = "image:\n  repository: rancher/fleet\n  tag: v0.17.0\n"
	fleetAnnotation = "- name: fleet\n  image: rancher/fleet:v0.17.0\n"
)

func TestFetchImagesV2AcceptsAChartWhoseAnnotationIsItsImages(t *testing.T) {
	root := t.TempDir()
	writeTestCharts(t, root, testChart{"fleet", "111.0.1", fleetValues, fleetAnnotation})

	imagesSet, err := fetchTestImagesOf(t, root, Linux)

	require.NoError(t, err)
	assert.Equal(t, map[string]map[string]struct{}{"rancher/fleet:v0.17.0": {"fleet:111.0.1": {}}}, imagesSet)
}

func TestFetchImagesV2FailsForAnImageThatTheChartDeclaresAndDoesNotHave(t *testing.T) {
	root := t.TempDir()
	writeTestCharts(t, root, testChart{"fleet", "111.0.1", fleetValues,
		fleetAnnotation + "- name: agent\n  image: rancher/fleet-agent:v0.17.0\n"})

	_, err := fetchTestImagesOf(t, root, Linux)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fleet:111.0.1")
	assert.Contains(t, err.Error(), "declared in helm.sh/images, not found in values.yaml: rancher/fleet-agent:v0.17.0")
}

func TestFetchImagesV2FailsForAnImageThatTheChartHasAndDoesNotDeclare(t *testing.T) {
	root := t.TempDir()
	values := fleetValues + "agent:\n  repository: rancher/fleet-agent\n  tag: v0.17.0\n"
	writeTestCharts(t, root, testChart{"fleet", "111.0.1", values, fleetAnnotation})

	_, err := fetchTestImagesOf(t, root, Linux)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fleet:111.0.1")
	assert.Contains(t, err.Error(), "found in values.yaml, not declared in helm.sh/images: rancher/fleet-agent:v0.17.0")
}

func TestFetchImagesV2FailsForAChartWithImagesAndNoAnnotation(t *testing.T) {
	root := t.TempDir()
	writeTestCharts(t, root, testChart{"fleet", "111.0.1", fleetValues, ""})

	_, err := fetchTestImagesOf(t, root, Linux)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "found in values.yaml, not declared in helm.sh/images: rancher/fleet:v0.17.0")
}

func TestFetchImagesV2AcceptsAChartWithNoImagesAndNoAnnotation(t *testing.T) {
	root := t.TempDir()
	writeTestCharts(t, root, testChart{"fleet-crd", "111.0.1", "replicas: 1\n", ""})

	imagesSet, err := fetchTestImagesOf(t, root, Linux)

	require.NoError(t, err)
	assert.Empty(t, imagesSet)
}

func TestFetchImagesV2FailsForAnAnnotationItCannotRead(t *testing.T) {
	root := t.TempDir()
	writeTestCharts(t, root, testChart{"fleet", "111.0.1", fleetValues, "image: [not, a, list\n"})

	_, err := fetchTestImagesOf(t, root, Linux)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fleet:111.0.1")
	assert.Contains(t, err.Error(), "helm.sh/images")
}

func TestFetchImagesV2ReportsTheMismatchesOfEveryChartAtOnce(t *testing.T) {
	root := t.TempDir()
	writeTestCharts(t, root,
		testChart{"fleet", "111.0.1", fleetValues, ""},
		testChart{"rancher-backup", "111.0.0", "image:\n  repository: rancher/backup-restore-operator\n  tag: v12.0.0\n",
			"- name: other\n  image: rancher/other:v1\n"},
	)

	_, err := fetchTestImagesOf(t, root, Linux)

	require.Error(t, err)
	for _, want := range []string{
		"fleet:111.0.1", "rancher/fleet:v0.17.0",
		"rancher-backup:111.0.0", "rancher/backup-restore-operator:v12.0.0", "rancher/other:v1",
	} {
		assert.Contains(t, err.Error(), want)
	}
}

func TestFetchImagesV2ComparesTheAnnotationWithTheImagesOfBothSystems(t *testing.T) {
	values := fleetValues + "win:\n  repository: rancher/fleet-agent-windows\n  tag: v0.17.0\n  os: windows\n"
	annotation := fleetAnnotation + "- name: win\n  image: rancher/fleet-agent-windows:v0.17.0\n"
	for name, osType := range map[string]OSType{"linux": Linux, "windows": Windows} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeTestCharts(t, root, testChart{"fleet", "111.0.1", values, annotation})

			imagesSet, err := fetchTestImagesOf(t, root, osType)

			require.NoError(t, err)
			if osType == Windows {
				assert.Equal(t, map[string]map[string]struct{}{
					"rancher/fleet-agent-windows:v0.17.0": {"fleet:111.0.1": {}}}, imagesSet)
			} else {
				assert.Equal(t, map[string]map[string]struct{}{
					"rancher/fleet:v0.17.0": {"fleet:111.0.1": {}}}, imagesSet)
			}
		})
	}
}

const dependencyAnnotation = fleetAnnotation + "- name: capi\n  dependency: cluster-api\n  image: rancher/cluster-api-controller:v1.14.4\n"

func TestFetchImagesV2AddsTheDependenciesOfAChartToTheImagesOfLinux(t *testing.T) {
	root := t.TempDir()
	writeTestCharts(t, root, testChart{"fleet", "111.0.1", fleetValues, dependencyAnnotation})

	imagesSet, err := fetchTestImagesOf(t, root, Linux)

	require.NoError(t, err, "a dependency is not in the values.yaml of the chart, which is why it is one")
	assert.Equal(t, map[string]map[string]struct{}{
		"rancher/fleet:v0.17.0":                  {"fleet:111.0.1": {}},
		"rancher/cluster-api-controller:v1.14.4": {"fleet:111.0.1": {}},
	}, imagesSet)
}

func TestFetchImagesV2DoesNotAddTheDependenciesOfAChartToTheImagesOfWindows(t *testing.T) {
	root := t.TempDir()
	writeTestCharts(t, root, testChart{"fleet", "111.0.1", fleetValues, dependencyAnnotation})

	imagesSet, err := fetchTestImagesOf(t, root, Windows)

	require.NoError(t, err)
	assert.Empty(t, imagesSet, "the annotation does not say that a dependency is for Windows")
}

func TestFetchImagesV2TakesAnyValueOfDependencyAsOne(t *testing.T) {
	for _, dependency := range []string{"true", "cluster-api", "'yes'"} {
		t.Run(dependency, func(t *testing.T) {
			root := t.TempDir()
			annotation := fleetAnnotation + "- name: capi\n  dependency: " + dependency + "\n  image: rancher/cluster-api-controller:v1.14.4\n"
			writeTestCharts(t, root, testChart{"fleet", "111.0.1", fleetValues, annotation})

			imagesSet, err := fetchTestImagesOf(t, root, Linux)

			require.NoError(t, err)
			assert.Contains(t, imagesSet, "rancher/cluster-api-controller:v1.14.4")
		})
	}
}

func TestFetchImagesV2DoesNotTakeFalseOrEmptyDependencyAsOne(t *testing.T) {
	for _, dependency := range []string{"false", "''"} {
		t.Run(dependency, func(t *testing.T) {
			root := t.TempDir()
			annotation := fleetAnnotation + "- name: capi\n  dependency: " + dependency + "\n  image: rancher/cluster-api-controller:v1.14.4\n"
			writeTestCharts(t, root, testChart{"fleet", "111.0.1", fleetValues, annotation})

			_, err := fetchTestImagesOf(t, root, Linux)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "declared in helm.sh/images, not found in values.yaml: rancher/cluster-api-controller:v1.14.4")
		})
	}
}

func TestFetchImagesV2ListsADependencyThatTheChartAlsoHasOnce(t *testing.T) {
	root := t.TempDir()
	annotation := "- name: fleet\n  dependency: itself\n  image: rancher/fleet:v0.17.0\n"
	writeTestCharts(t, root, testChart{"fleet", "111.0.1", fleetValues, annotation})

	imagesSet, err := fetchTestImagesOf(t, root, Linux)

	require.NoError(t, err)
	assert.Equal(t, map[string]map[string]struct{}{"rancher/fleet:v0.17.0": {"fleet:111.0.1": {}}}, imagesSet)
}

func TestFetchImagesV2StillFailsForAnImageThatIsNotADependencyWhenThereAreDependencies(t *testing.T) {
	root := t.TempDir()
	writeTestCharts(t, root, testChart{"fleet", "111.0.1", fleetValues,
		dependencyAnnotation + "- name: gone\n  image: rancher/gone:v1\n"})

	_, err := fetchTestImagesOf(t, root, Linux)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rancher/gone:v1")
	assert.NotContains(t, err.Error(), "cluster-api-controller", "a dependency is not a difference")
}
