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

// writeTestIndex writes an index.yaml with a single chart version whose only URL is url, and whose
// helm.sh/images annotation lists the image of testChartValues.
func writeTestIndex(t *testing.T, root, url string) {
	t.Helper()
	index := fmt.Sprintf(`apiVersion: v1
entries:
  %[1]s:
  - annotations:
      catalog.cattle.io/rancher-version: '>= 2.16.0-0 < 2.17.0-0'
      helm.sh/images: |
        - name: %[1]s
          image: rancher/fleet:v0.17.0
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

type fetchImages func(Charts, map[string]map[string]struct{}) error

// fetchers are the two ways of reading a charts repository: the one of the old system and the one of the catalog.
var fetchers = map[string]fetchImages{
	"FetchImages":   Charts.FetchImages,
	"FetchImagesV2": Charts.FetchImagesV2,
}

func fetchTestImages(t *testing.T, fetch fetchImages, root string) (map[string]map[string]struct{}, error) {
	t.Helper()
	imagesSet := map[string]map[string]struct{}{}
	charts := Charts{ExportConfig{ChartsPath: root, RancherVersion: "2.16.99", OsType: Linux}}
	err := fetch(charts, imagesSet)
	return imagesSet, err
}

func TestFetchImagesChartWithRelativeURL(t *testing.T) {
	for name, fetch := range fetchers {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			archive := "assets/fleet/fleet-111.0.1+up0.17.0.tgz"
			writeTestIndex(t, root, archive)
			writeTestArchive(t, root, archive, testChartValues)

			imagesSet, err := fetchTestImages(t, fetch, root)

			require.NoError(t, err)
			assert.Equal(t, map[string]map[string]struct{}{
				"rancher/fleet:v0.17.0": {"fleet:111.0.1+up0.17.0": {}},
			}, imagesSet)
		})
	}
}

func TestFetchImagesChartWithRelativeURLAndNoArchiveIsSkipped(t *testing.T) {
	for name, fetch := range fetchers {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeTestIndex(t, root, "assets/fleet/fleet-111.0.1+up0.17.0.tgz")

			imagesSet, err := fetchTestImages(t, fetch, root)

			require.NoError(t, err)
			assert.Empty(t, imagesSet)
		})
	}
}

func TestFetchImagesV2ChartWithRemoteURLReadsArchiveFromAssets(t *testing.T) {
	for _, url := range []string{
		"oci://registry.example.com/rancher/catalog/fleet:111.0.1_up0.17.0@sha256:0123456789abcdef",
		"https://charts.example.com/fleet-111.0.1+up0.17.0.tgz",
	} {
		t.Run(url, func(t *testing.T) {
			root := t.TempDir()
			writeTestIndex(t, root, url)
			writeTestArchive(t, root, "assets/fleet/fleet-111.0.1+up0.17.0.tgz", testChartValues)

			imagesSet, err := fetchTestImages(t, Charts.FetchImagesV2, root)

			require.NoError(t, err)
			assert.Equal(t, map[string]map[string]struct{}{
				"rancher/fleet:v0.17.0": {"fleet:111.0.1+up0.17.0": {}},
			}, imagesSet)
		})
	}
}

func TestFetchImagesV2ChartWithRemoteURLAndNoArchiveFails(t *testing.T) {
	root := t.TempDir()
	writeTestIndex(t, root, "oci://registry.example.com/rancher/catalog/fleet:111.0.1_up0.17.0@sha256:0123456789abcdef")

	imagesSet, err := fetchTestImages(t, Charts.FetchImagesV2, root)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "assets/fleet/fleet-111.0.1+up0.17.0.tgz")
	assert.Empty(t, imagesSet)
}

// testVersion is a version of the test chart in an index that has several of them: the Rancher versions it
// supports, and the image its values.yaml has.
type testVersion struct {
	version        string
	rancherVersion string
	imageTag       string
}

func (v testVersion) archive() string {
	return fmt.Sprintf("assets/%[1]s/%[1]s-%[2]s.tgz", testChartName, v.version)
}

// writeTestIndexOfVersions writes an index.yaml with the given versions of the test chart, listed in the given
// order, each with its archive in assets/, an oci:// URL, and a helm.sh/images annotation with its image.
func writeTestIndexOfVersions(t *testing.T, root string, versions []testVersion) {
	t.Helper()
	index := "apiVersion: v1\nentries:\n  " + testChartName + ":\n"
	for _, v := range versions {
		index += fmt.Sprintf(`  - annotations:
      catalog.cattle.io/rancher-version: '%[3]s'
      helm.sh/images: |
        - name: %[1]s
          image: rancher/fleet:%[4]s
    apiVersion: v2
    name: %[1]s
    version: %[2]s
    urls:
    - oci://registry.example.com/rancher/catalog/%[1]s:%[2]s@sha256:0123456789abcdef
`, testChartName, v.version, v.rancherVersion, v.imageTag)
		writeTestArchive(t, root, v.archive(),
			fmt.Sprintf("image:\n  repository: rancher/fleet\n  tag: %s\n", v.imageTag))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.yaml"), []byte(index), 0o644))
}

// versionsOfSeveralMinors is what the staging branch of rancher/charts has for a chart that is released for more
// than one Rancher minor: the newest version is for the next minor.
var versionsOfSeveralMinors = []testVersion{
	{"112.0.0+up0.13.0", ">= 2.17.0-0 < 2.18.0-0", "v0.13.0"},
	{"111.0.1+up0.12.1", ">= 2.16.0-0 < 2.17.0-0", "v0.12.1"},
	{"111.0.0+up0.12.0", ">= 2.16.0-0 < 2.17.0-0", "v0.12.0"},
	{"110.0.0+up0.11.0", ">= 2.15.0-0 < 2.16.0-0", "v0.11.0"},
}

func TestFetchImagesV2TakesTheNewestVersionOfTheMinor(t *testing.T) {
	for _, tc := range []struct {
		rancherVersion string
		wantImage      string
		wantSource     string
	}{
		{"2.16.99", "rancher/fleet:v0.12.1", "fleet:111.0.1+up0.12.1"},
		{"2.16.0", "rancher/fleet:v0.12.1", "fleet:111.0.1+up0.12.1"},
		{"2.16.0-rc1", "rancher/fleet:v0.12.1", "fleet:111.0.1+up0.12.1"},
		{"2.17.0", "rancher/fleet:v0.13.0", "fleet:112.0.0+up0.13.0"},
		{"2.15.4", "rancher/fleet:v0.11.0", "fleet:110.0.0+up0.11.0"},
	} {
		t.Run(tc.rancherVersion, func(t *testing.T) {
			root := t.TempDir()
			writeTestIndexOfVersions(t, root, versionsOfSeveralMinors)
			imagesSet := map[string]map[string]struct{}{}
			charts := Charts{ExportConfig{ChartsPath: root, RancherVersion: tc.rancherVersion, OsType: Linux}}

			require.NoError(t, charts.FetchImagesV2(imagesSet))

			assert.Equal(t, map[string]map[string]struct{}{tc.wantImage: {tc.wantSource: {}}}, imagesSet)
		})
	}
}

func TestFetchImagesV2TakesTheNewestVersionOfTheMinorWhateverTheOrderOfTheIndex(t *testing.T) {
	root := t.TempDir()
	reversed := make([]testVersion, 0, len(versionsOfSeveralMinors))
	for i := len(versionsOfSeveralMinors) - 1; i >= 0; i-- {
		reversed = append(reversed, versionsOfSeveralMinors[i])
	}
	writeTestIndexOfVersions(t, root, reversed)
	imagesSet := map[string]map[string]struct{}{}
	charts := Charts{ExportConfig{ChartsPath: root, RancherVersion: "2.16.99", OsType: Linux}}

	require.NoError(t, charts.FetchImagesV2(imagesSet))

	assert.Equal(t, map[string]map[string]struct{}{
		"rancher/fleet:v0.12.1": {"fleet:111.0.1+up0.12.1": {}},
	}, imagesSet)
}

func TestFetchImagesV2LeavesOutAChartWithNoVersionOfTheMinor(t *testing.T) {
	root := t.TempDir()
	writeTestIndexOfVersions(t, root, versionsOfSeveralMinors)
	imagesSet := map[string]map[string]struct{}{}
	charts := Charts{ExportConfig{ChartsPath: root, RancherVersion: "2.18.0", OsType: Linux}}

	require.NoError(t, charts.FetchImagesV2(imagesSet))

	assert.Empty(t, imagesSet)
}
