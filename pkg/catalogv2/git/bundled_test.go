package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	bundledNamespace = ""
	bundledName      = "fake-charts"
	bundledURL       = "https://git.fake.io/charts"
)

// Test_Ensure_Bundled_NotCheckedOut covers Ensure against the repository as the images ship it: a
// clone with no working tree, holding the single commit it was bundled with, and no way to reach
// upstream.
func Test_Ensure_Bundled_NotCheckedOut(t *testing.T) {
	testCases := []struct {
		test          string
		systemCatalog string
		commit        string
		expectedError string
	}{
		{
			test:          "#1 TestCase: Success - Bundled Commit Is Checked Out Without Reaching Upstream",
			systemCatalog: "bundled",
			commit:        "v1",
		},
		{
			test:          "#2 TestCase: Success - Old Commit From Previous Version Ignored, Uses Bundled HEAD",
			systemCatalog: "bundled",
			commit:        "missing-commit",
			expectedError: "",
		},
		{
			test:          "#3 TestCase: Failure - Non-Bundled Mode With Missing Commit Tries To Fetch",
			systemCatalog: "external",
			commit:        "missing-commit",
			expectedError: "ensure failure",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			setSystemCatalog(t, tc.systemCatalog)
			f := newFixture(t)

			err := Ensure(nil, bundledNamespace, bundledName, bundledURL, f.commits[tc.commit], false, nil)

			if tc.expectedError != "" {
				require.ErrorContains(t, err, tc.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, f.commits["v1"], f.headCommit(t, f.dir), "should be at bundled HEAD commit")
				chart, err := os.ReadFile(filepath.Join(f.dir, "chart.yaml"))
				require.NoError(t, err)
				assert.Equal(t, "v1", string(chart), "the working tree should have been restored")
			}
		})
	}
}

// Test_Update_Bundled_NotCheckedOut covers the same shipped state through Update.
func Test_Update_Bundled_NotCheckedOut(t *testing.T) {
	testCases := []struct {
		test           string
		systemCatalog  string
		branch         string
		expectedCommit string
	}{
		{
			test:           "#1 TestCase: Success - Working Tree Restored Without Reaching Upstream",
			systemCatalog:  "bundled",
			branch:         "main",
			expectedCommit: "v1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			setSystemCatalog(t, tc.systemCatalog)
			f := newFixture(t)

			commit, err := Update(nil, bundledNamespace, bundledName, bundledURL, tc.branch, false, nil)

			require.NoError(t, err)
			assert.Equal(t, f.commits[tc.expectedCommit], commit, "unexpected commit returned")
			chart, err := os.ReadFile(filepath.Join(f.dir, "chart.yaml"))
			require.NoError(t, err)
			assert.Equal(t, tc.expectedCommit, string(chart), "the working tree should have been restored")
		})
	}
}

// Test_Ensure_Bundled_UpgradeScenario_ShallowClone tests upgrade where old commit is not in history.
// Simulates upgrading from v2.14.4 (commit v1) to v2.14.5 (commit v2) with --depth 1 clone.
// Should use HEAD (v2) instead of trying to fetch the old v1 commit.
func Test_Ensure_Bundled_UpgradeScenario_ShallowClone(t *testing.T) {
	setSystemCatalog(t, "bundled")
	f := newWorkspace(t)

	f.commits["v2"] = f.mockCommitRemote(t, "v2")

	require.NoError(t, f.git.gitCmd(nil, "clone", "--no-checkout", "--depth", "1", "--branch", "main",
		"--", "file://"+f.mockRemote, f.dir))

	require.Equal(t, f.commits["v2"], f.headCommit(t, f.dir), "should have v2 after clone")

	require.NoError(t, os.RemoveAll(f.mockRemote))

	err := Ensure(nil, bundledNamespace, bundledName, bundledURL, f.commits["v1"], false, nil)

	require.NoError(t, err, "should not error in airgap with old commit")
	assert.Equal(t, f.commits["v2"], f.headCommit(t, f.dir), "should be at bundled v2 commit")

	chart, err := os.ReadFile(filepath.Join(f.dir, "chart.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "v2", string(chart), "working tree should be at v2")
}

// Test_Ensure_Bundled_UpgradeScenario_FullClone tests upgrade where old commit IS in history.
// Without the fix, reset to old commit would succeed and silently revert charts to old version.
// Simulates upgrading from v2.14.4 (commit v1) to v2.14.5 (commit v2) where bundled repo has
// full history with both commits. Should use HEAD (v2) instead of reverting to v1.
func Test_Ensure_Bundled_UpgradeScenario_FullClone(t *testing.T) {
	setSystemCatalog(t, "bundled")
	f := newWorkspace(t)

	f.commits["v2"] = f.mockCommitRemote(t, "v2")

	require.NoError(t, f.git.gitCmd(nil, "clone", "--no-checkout", "--branch", "main",
		"--", "file://"+f.mockRemote, f.dir))

	require.Equal(t, f.commits["v2"], f.headCommit(t, f.dir), "should have v2 as HEAD after clone")

	_, err := f.git.gitOutput("-C", f.dir, "rev-parse", f.commits["v1"])
	require.NoError(t, err, "v1 should exist in bundled repo history")

	require.NoError(t, os.RemoveAll(f.mockRemote))

	err = Ensure(nil, bundledNamespace, bundledName, bundledURL, f.commits["v1"], false, nil)

	require.NoError(t, err, "should not error")
	assert.Equal(t, f.commits["v2"], f.headCommit(t, f.dir), "should stay at v2, not revert to v1")

	chart, err := os.ReadFile(filepath.Join(f.dir, "chart.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "v2", string(chart), "working tree should be at v2")
}

// fixture is the repository a test case runs against.
type fixture struct {
	dir        string
	mockRemote string
	commits    map[string]string
	git        *git
}

// newWorkspace moves the test into a temporary working directory - so that the relative directories
// RepoDir resolves to (localDir and stateDir) end up inside it instead of polluting the source
// tree - and creates the bundled catalog directory along with the upstream repository the clone
// comes from, holding the commits "missing-commit" and "v1". The two constructors below clone it into
// that directory, each leaving the clone in a different state.
//
// Every commit writes its own name into chart.yaml, so the file tells which commit is checked out.
func newWorkspace(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	require.NoError(t, os.MkdirAll(work, 0o755))
	t.Chdir(work)

	g := createBundledGitDirectory(t)

	f := &fixture{
		dir:        g.Directory,
		mockRemote: filepath.Join(root, "upstream"),
		commits:    map[string]string{},
		git:        g,
	}
	require.NoError(t, os.MkdirAll(f.mockRemote, 0o755))
	require.NoError(t, f.git.gitCmd(nil, "-C", f.mockRemote, "init", "-b", "main"))
	f.commits["missing-commit"] = f.mockCommitRemote(t, "missing-commit")
	f.commits["v1"] = f.mockCommitRemote(t, "v1")

	return f
}

func createBundledGitDirectory(t *testing.T) *git {
	require.NoError(t, os.MkdirAll(filepath.Join(localDir, bundledNamespace, bundledName, Hash(bundledURL)), 0o755))
	g, err := gitForRepo(nil, bundledNamespace, bundledName, bundledURL, false, nil)
	require.NoError(t, err)
	require.True(t, IsBundled(g.Directory))
	require.Equal(t, filepath.Join(localDir, bundledName, Hash(bundledURL)), g.Directory)

	return g
}

// newFixture clones the upstream the way package/Dockerfile does, with --no-checkout and
// --depth 1, leaving a .git holding only "v1" and no working tree at all.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := newWorkspace(t)

	// a file:// URL rather than a path, git ignores --depth on a plain local clone
	require.NoError(t, f.git.gitCmd(nil, "clone", "--no-checkout", "--depth", "1", "--branch", "main",
		"--", "file://"+f.mockRemote, f.dir))
	require.NoFileExists(t, filepath.Join(f.dir, "chart.yaml"), "the clone should have no working tree")
	depth, err := f.git.gitOutput("-C", f.dir, "rev-list", "--count", "HEAD")
	require.NoError(t, err)
	require.Equal(t, "1", depth, `the clone should hold "v1" and nothing else`)
	require.NoError(t, os.RemoveAll(f.mockRemote))

	return f
}

// mockCommitRemote writes the commit name into chart.yaml in the mockRemote repository and commits it.
func (f *fixture) mockCommitRemote(t *testing.T, name string) string {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(f.mockRemote, "chart.yaml"), []byte(name), 0o644))
	require.NoError(t, f.git.gitCmd(nil, "-C", f.mockRemote, "add", "chart.yaml"))
	require.NoError(t, f.git.gitCmd(nil, "-C", f.mockRemote, "-c", "user.name=rancher",
		"-c", "user.email=rancher@suse.com", "-c", "commit.gpgsign=false", "commit", "-m", name))
	return f.headCommit(t, f.mockRemote)
}

// headCommit returns the commit checked out in dir.
func (f *fixture) headCommit(t *testing.T, dir string) string {
	t.Helper()
	commit, err := f.git.gitOutput("-C", dir, "rev-parse", "HEAD")
	require.NoError(t, err)
	return commit
}

func setSystemCatalog(t *testing.T, value string) {
	t.Helper()
	previous := settings.SystemCatalog.Get()
	require.NoError(t, settings.SystemCatalog.Set(value))
	t.Cleanup(func() {
		require.NoError(t, settings.SystemCatalog.Set(previous))
	})
}
