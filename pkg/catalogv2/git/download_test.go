package git

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

const (
	chartsSmallForkURL = "https://github.com/rancher/charts-small-fork"
	mainBranch         = "main"
	lastBranch         = "test-1"
)

func TestMain(m *testing.M) {
	// Run all the tests
	exitCode := m.Run()

	// Cleanup after tests
	cleanup()

	// Exit with the proper code
	os.Exit(exitCode)
}

func cleanup() {
	// Delete the management-state directory
	os.RemoveAll("management-state")
}

func Test_Ensure(t *testing.T) {
	testCases := []struct {
		test            string
		secret          *corev1.Secret
		namespace       string
		name            string
		gitURL          string
		commit          string
		insecureSkipTLS bool
		caBundle        []byte
		branch          string
		expectedError   error
	}{
		{
			test:            "#1 TestCase: Success - Clone, Reset And Exit",
			secret:          nil,
			namespace:       "cattle-test",
			name:            "small-fork-test",
			gitURL:          chartsSmallForkURL,
			commit:          "0e2b9da9ddde5c1e502bba6474119856496e5026",
			insecureSkipTLS: false,
			caBundle:        []byte{},
			branch:          mainBranch,
			expectedError:   nil,
		},
		{
			test:            "#2 TestCase: Success - Clone, Reset And Fetch Last Branch",
			secret:          nil,
			namespace:       "cattle-test",
			name:            "small-fork-test",
			gitURL:          chartsSmallForkURL,
			commit:          "0e2b9da9ddde5c1e502bba6474119856496e5026",
			insecureSkipTLS: false,
			caBundle:        []byte{},
			branch:          lastBranch,
			expectedError:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Ensure(tc.secret, tc.namespace, tc.name, tc.gitURL, tc.commit, tc.insecureSkipTLS, tc.caBundle)
			// Check the error
			if tc.expectedError == nil && tc.expectedError != err {
				t.Errorf("Expected error: %v |But got: %v", tc.expectedError, err)
			}

			// Check the error
			if tc.expectedError == nil && tc.expectedError != err {
				t.Errorf("Expected error: %v |But got: %v", tc.expectedError, err)
			}
			// Only testing error in some cases
			if err != nil {
				assert.EqualError(t, tc.expectedError, err.Error())
			}
		})
	}
}

func Test_Head(t *testing.T) {
	testCases := []struct {
		test            string
		secret          *corev1.Secret
		namespace       string
		name            string
		gitURL          string
		insecureSkipTLS bool
		caBundle        []byte
		branch          string
		expectedCommit  string
		expectedError   error
	}{
		{
			test:            "#1 TestCase: Success - Clone, Reset And Return Commit",
			secret:          nil,
			namespace:       "cattle-test",
			name:            "small-fork-test",
			gitURL:          chartsSmallForkURL,
			insecureSkipTLS: false,
			caBundle:        []byte{},
			branch:          mainBranch,
			expectedCommit:  "226d544def39de56db210e96d2b0b535badf9bdd",
			expectedError:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			commit, err := Head(tc.secret, tc.namespace, tc.name, tc.gitURL, tc.branch, tc.insecureSkipTLS, tc.caBundle)
			// Check the error
			if tc.expectedError == nil && tc.expectedError != err {
				t.Errorf("Expected error: %v |But got: %v", tc.expectedError, err)
			}
			// Only testing error in some cases
			if err != nil {
				assert.EqualError(t, tc.expectedError, err.Error())
			}

			assert.Equal(t, len(commit), len(tc.expectedCommit))
		})
	}
}

func Test_Update(t *testing.T) {
	testCases := []struct {
		test              string
		secret            *corev1.Secret
		namespace         string
		name              string
		gitURL            string
		insecureSkipTLS   bool
		caBundle          []byte
		branch            string
		systemCatalogMode string
		expectedCommit    string
		expectedError     string
		dir               string
	}{
		{
			test:              "#1 TestCase: Success ",
			secret:            nil,
			namespace:         "cattle-test",
			name:              "small-fork-test",
			gitURL:            chartsSmallForkURL,
			insecureSkipTLS:   false,
			caBundle:          []byte{},
			branch:            lastBranch,
			systemCatalogMode: "",
			expectedCommit:    "226d544def39de56db210e96d2b0b535badf9bdd",
			expectedError:     "",
		},
		{
			test:              "Returns an error if invalid branch is specified",
			secret:            nil,
			namespace:         "cattle-test",
			name:              "small-fork-test",
			gitURL:            chartsSmallForkURL,
			insecureSkipTLS:   false,
			caBundle:          []byte{},
			branch:            "invalidbranch",
			systemCatalogMode: "",
			expectedCommit:    "226d544def39de56db210e96d2b0b535badf9bdd",
			expectedError:     "Remote branch invalidbranch not found in upstream origin",
			dir:               fmt.Sprintf("%s/%s", localDir, "cattle-test/small-fork-test/d39a2f6abd49e537e5015bbe1a4cd4f14919ba1c3353208a7ff6be37ffe00c52"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.dir != "" {
				err := os.MkdirAll(tc.dir, 0o755)
				assert.NoError(t, err)
			}

			commit, err := Update(tc.secret, tc.namespace, tc.name, tc.gitURL, tc.branch, tc.insecureSkipTLS, tc.caBundle)
			if tc.expectedError != "" {
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(commit), len(tc.expectedCommit))
			}
		})
	}
}

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
			test:          "#2 TestCase: Failure - Any Other Commit Has To Be Fetched",
			systemCatalog: "bundled",
			commit:        "missing-commit", // outside the --depth 1 clone, so it can only come from upstream
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
			}
			// either way the repository is left on the bundled commit, the reset to HEAD comes first
			assert.Equal(t, f.commits["v1"], f.headCommit(t, f.dir), "unexpected commit checked out")
			chart, err := os.ReadFile(filepath.Join(f.dir, "chart.yaml"))
			require.NoError(t, err)
			assert.Equal(t, "v1", string(chart), "the working tree should have been restored")
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
