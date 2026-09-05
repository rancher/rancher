package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testIndexYAML = `apiVersion: v1
entries:
  rancher-webhook:
  - apiVersion: v2
    name: rancher-webhook
    version: 111.0.0+up0.12.1-rc.3
    urls:
    - assets/rancher-webhook/rancher-webhook-111.0.0+up0.12.1-rc.3.tgz
generated: "2026-01-01T00:00:00Z"
`

// writeChart lays down a chart directory the tree walk would otherwise load, re-tar and digest.
func writeChart(t *testing.T, dir, name, version string) {
	t.Helper()
	chartDir := filepath.Join(dir, "charts", name, version)
	require.NoError(t, os.MkdirAll(chartDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"),
		[]byte("apiVersion: v2\nname: "+name+"\nversion: "+version+"\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("{}\n"), 0644))
}

func TestBuildOrGetIndex_UsesRootIndex(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.yaml"), []byte(testIndexYAML), 0644))
	// A chart the walk would have processed. If the root index is honoured, it is never read, so
	// the returned index must contain only what index.yaml declares.
	writeChart(t, dir, "some-other-chart", "1.2.3")

	index, err := buildOrGetIndex(dir)
	require.NoError(t, err)
	require.NotNil(t, index)

	assert.Contains(t, index.Entries, "rancher-webhook")
	assert.NotContains(t, index.Entries, "some-other-chart",
		"the root index.yaml should be returned as-is, without walking the chart tree")

	versions := index.Entries["rancher-webhook"]
	require.Len(t, versions, 1)
	assert.Equal(t, "111.0.0+up0.12.1-rc.3", versions[0].Version)
}

// makeChartUnreadable makes a chart's values.yaml unreadable. ensureNoSymlinks only stats each
// entry so it still passes, but loader.LoadDir has to read the file, so the chart tree walk
// fails. That gives the tests below a way to observe whether the walk ran at all.
func makeChartUnreadable(t *testing.T, dir, name, version string) {
	t.Helper()
	path := filepath.Join(dir, "charts", name, version, "values.yaml")
	require.NoError(t, os.Chmod(path, 0000))
	t.Cleanup(func() { _ = os.Chmod(path, 0644) })
}

// TestBuildOrGetIndex_SkipsChartTree is the point of the change: with a root index.yaml present
// the chart tree must not be read at all. The companion test below confirms this same tree does
// break the walk, so success here means the walk genuinely did not run.
func TestBuildOrGetIndex_SkipsChartTree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, permission bits are not enforced")
	}

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.yaml"), []byte(testIndexYAML), 0644))
	writeChart(t, dir, "some-other-chart", "1.2.3")
	makeChartUnreadable(t, dir, "some-other-chart", "1.2.3")

	index, err := buildOrGetIndex(dir)
	require.NoError(t, err, "the chart tree was read even though a root index.yaml exists")
	require.NotNil(t, index)
	assert.Contains(t, index.Entries, "rancher-webhook")
}

// TestBuildOrGetIndex_WalkReadsChartTree is the control for the test above: the same unreadable
// chart makes buildOrGetIndex fail when there is no root index.yaml to short circuit on.
func TestBuildOrGetIndex_WalkReadsChartTree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, permission bits are not enforced")
	}

	dir := t.TempDir()
	writeChart(t, dir, "some-other-chart", "1.2.3")
	makeChartUnreadable(t, dir, "some-other-chart", "1.2.3")

	_, err := buildOrGetIndex(dir)
	require.Error(t, err, "expected the chart tree walk to read the chart and fail")
}

func TestBuildOrGetIndex_FallsBackToWalk(t *testing.T) {
	dir := t.TempDir()
	// No index.yaml at all: the index has to be built from the chart tree.
	writeChart(t, dir, "some-other-chart", "1.2.3")

	index, err := buildOrGetIndex(dir)
	require.NoError(t, err)
	require.NotNil(t, index)

	assert.Contains(t, index.Entries, "some-other-chart",
		"with no root index.yaml the chart tree must still be walked")
}

func TestBuildOrGetIndex_FallsBackWhenRootIndexMalformed(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.yaml"), []byte("\t\tnot: [valid"), 0644))
	writeChart(t, dir, "some-other-chart", "1.2.3")

	// Matches how the walk already treats an index.yaml it cannot load: ignore it and build.
	index, err := buildOrGetIndex(dir)
	require.NoError(t, err)
	require.NotNil(t, index)
	assert.Contains(t, index.Entries, "some-other-chart")
}

func TestBuildOrGetIndex_RejectsSymlinks(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.yaml"), []byte(testIndexYAML), 0644))

	target := filepath.Join(t.TempDir(), "outside.yaml")
	require.NoError(t, os.WriteFile(target, []byte(testIndexYAML), 0644))
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "sneaky.yaml")))

	// The symlink check must still run ahead of the root-index short circuit.
	_, err := buildOrGetIndex(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink found")
}

func TestRootIndex(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		assert.Nil(t, rootIndex(t.TempDir()))
	})

	t.Run("directory instead of file", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "index.yaml"), 0755))
		assert.Nil(t, rootIndex(dir))
	})

	t.Run("present", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "index.yaml"), []byte(testIndexYAML), 0644))
		index := rootIndex(dir)
		require.NotNil(t, index)
		assert.Contains(t, index.Entries, "rancher-webhook")
	})
}
