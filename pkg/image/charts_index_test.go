package image

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testChartName    = "fleet"
	testChartVersion = "111.0.1+up0.17.0"
	testChartValues  = "image:\n  repository: rancher/fleet\n  tag: v0.17.0\n"
)

// writeTestIndex writes an index.yaml with a single chart version whose only URL is url.
func writeTestIndex(t *testing.T, root, url string) {
	t.Helper()
	index := fmt.Sprintf(`apiVersion: v1
entries:
  %[1]s:
  - annotations:
      catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'
    apiVersion: v2
    name: %[1]s
    version: %[2]s
    urls:
    - %[3]s
`, testChartName, testChartVersion, url)
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.yaml"), []byte(index), 0o644))
}

// writeTestArchive packs a chart with the given values.yaml at path, relative to root.
func writeTestArchive(t *testing.T, root, path, values string) {
	t.Helper()
	file := filepath.Join(root, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o755))
	f, err := os.Create(file)
	require.NoError(t, err)
	defer f.Close()
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     testChartName + "/values.yaml",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(values)),
	}))
	_, err = tw.Write([]byte(values))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gzw.Close())
}

func fetchTestImages(t *testing.T, root string) (map[string]map[string]struct{}, error) {
	t.Helper()
	imagesSet := map[string]map[string]struct{}{}
	charts := Charts{ExportConfig{ChartsPath: root, RancherVersion: "2.16.99", OsType: Linux}}
	err := charts.FetchImages(imagesSet)
	return imagesSet, err
}

func TestFetchImagesChartWithRelativeURL(t *testing.T) {
	root := t.TempDir()
	archive := "assets/fleet/fleet-111.0.1+up0.17.0.tgz"
	writeTestIndex(t, root, archive)
	writeTestArchive(t, root, archive, testChartValues)

	imagesSet, err := fetchTestImages(t, root)

	require.NoError(t, err)
	assert.Equal(t, map[string]map[string]struct{}{
		"rancher/fleet:v0.17.0": {"fleet:111.0.1+up0.17.0": {}},
	}, imagesSet)
}

func TestFetchImagesChartWithRelativeURLAndNoArchiveIsSkipped(t *testing.T) {
	root := t.TempDir()
	writeTestIndex(t, root, "assets/fleet/fleet-111.0.1+up0.17.0.tgz")

	imagesSet, err := fetchTestImages(t, root)

	require.NoError(t, err)
	assert.Empty(t, imagesSet)
}
