package image

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeChartTgz writes a chart archive containing the given files to a temporary directory and returns its path.
func writeChartTgz(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chart.tgz")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	gzw := gzip.NewWriter(f)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(content)), Typeflag: tar.TypeReg}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	return path
}

// TestImagesFromDecodedValues decodes real values.yaml files from a chart archive and picks images from them. Unlike
// TestPickImagesFromValuesMap, which builds the values maps by hand, it catches changes in the map types produced by
// the YAML decoder, such as the ones introduced by go.yaml.in/yaml/v3 and later, where nested mappings decode to
// map[string]interface{} instead of map[interface{}]interface{}.
func TestImagesFromDecodedValues(t *testing.T) {
	tgz := writeChartTgz(t, map[string]string{
		"my-chart/Chart.yaml": "name: my-chart\nversion: 1.0.0\n",
		"my-chart/values.yaml": `
image:
  repository: rancher/top-level
  tag: v1.0.0
controller:
  image:
    repository: rancher/nested
    tag: v2.0.0
sidecars:
  - name: one
    image:
      repository: rancher/in-list
      tag: 3.1
windows:
  image:
    repository: rancher/windows-only
    tag: v4.0.0
    os: windows
1: a mapping with a non-string key
mixedKeys:
  true: also a non-string key
  image:
    repository: rancher/under-mixed-keys
    tag: v5.0.0
`,
		"my-chart/charts/sub-chart/values.yaml": `
deep:
  deeper:
    image:
      repository: rancher/sub-chart
      tag: v6.0.0
`,
		"my-chart/templates/deployment.yaml": "image: rancher/not-a-values-file:v0\n",
	})

	valuesSlice, err := decodeValuesFilesInTgz(tgz)
	require.NoError(t, err)
	require.Len(t, valuesSlice, 2)

	images := map[string]map[string]struct{}{}
	for _, values := range valuesSlice {
		require.NoError(t, pickImagesFromValuesMap(images, values, "my-chart:1.0.0", Linux, ""))
	}
	source := map[string]struct{}{"my-chart:1.0.0": {}}
	require.Equal(t, map[string]map[string]struct{}{
		"rancher/top-level:v1.0.0":        source,
		"rancher/nested:v2.0.0":           source,
		"rancher/in-list:3.1":             source,
		"rancher/under-mixed-keys:v5.0.0": source,
		"rancher/sub-chart:v6.0.0":        source,
	}, images)
}
