package git

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

// Test_CheckoutState covers the gate in front of Chart and Icon: a bundled clone ships without a
// working tree, so nothing may be served from it until Ensure, Head or Update restored it.
func Test_CheckoutState(t *testing.T) {
	chartVersion := &repo.ChartVersion{
		Metadata: &chart.Metadata{Name: "chart", Version: "1.0.0", Icon: "chart.yaml"},
		URLs:     []string{"chart.yaml"},
	}

	assertGate := func(t *testing.T, notCheckedOut bool) {
		t.Helper()
		_, _, iconErr := Icon(bundledNamespace, bundledName, bundledURL, chartVersion)
		_, chartErr := Chart(bundledNamespace, bundledName, bundledURL, chartVersion)
		assert.Equal(t, !notCheckedOut, IsCheckedOut(bundledName, bundledURL))
		assert.Equal(t, notCheckedOut, errors.Is(iconErr, ErrNotCheckedOut), "icon: %v", iconErr)
		assert.Equal(t, notCheckedOut, errors.Is(chartErr, ErrNotCheckedOut), "chart: %v", chartErr)
	}

	testCases := []struct {
		test    string
		restore func(t *testing.T) error
	}{
		{
			test: "#1 TestCase: Ensure Restores The Working Tree",
			restore: func(t *testing.T) error {
				return Ensure(nil, bundledNamespace, bundledName, bundledURL, "", false, nil)
			},
		},
		{
			test: "#2 TestCase: Head Restores The Working Tree",
			restore: func(t *testing.T) error {
				_, err := Head(nil, bundledNamespace, bundledName, bundledURL, "main", false, nil)
				return err
			},
		},
		{
			test: "#3 TestCase: Update Restores The Working Tree",
			restore: func(t *testing.T) error {
				_, err := Update(nil, bundledNamespace, bundledName, bundledURL, "main", false, nil)
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			setSystemCatalog(t, "bundled")
			newFixture(t)

			assertGate(t, true)
			require.NoError(t, tc.restore(t))
			assertGate(t, false)

			ForgetRepo(bundledName)
			assertGate(t, true)
		})
	}
}

// Test_ForgetRepoAfterRemoval covers the order used when a ClusterRepo is deleted: the directory is
// removed before the checkout state is dropped, and the state must go with it even though the
// bundled directory it was recorded under no longer exists.
func Test_ForgetRepoAfterRemoval(t *testing.T) {
	setSystemCatalog(t, "bundled")
	newFixture(t)

	require.NoError(t, Ensure(nil, bundledNamespace, bundledName, bundledURL, "", false, nil))
	require.True(t, IsCheckedOut(bundledName, bundledURL))

	// IsCheckedOut can't be used to tell: once the bundled directory is gone it resolves to the state
	// directory and reports false whatever is left behind, so look at the recorded key.
	key := checkoutKey(RepoDir(bundledNamespace, bundledName, bundledURL))
	parent := ParentRepoDir(bundledNamespace, bundledName)
	require.NoError(t, os.RemoveAll(parent))
	ForgetRepo(bundledName)

	_, stale := checkedOut.Load(key)
	assert.False(t, stale, "the checkout state of the removed directory should have been dropped")
}

// Test_WorktreePopulated covers telling a restored working tree from the states a repository can be
// in when this process hasn't restored it itself.
func Test_WorktreePopulated(t *testing.T) {
	testCases := []struct {
		test     string
		prepare  func(t *testing.T, f *fixture)
		expected bool
	}{
		{
			test:     "#1 TestCase: Cloned Without Checkout",
			prepare:  func(t *testing.T, f *fixture) {},
			expected: false,
		},
		{
			test: "#2 TestCase: Shipped With Its Files",
			prepare: func(t *testing.T, f *fixture) {
				require.NoError(t, gitCmd("-C", f.dir, "checkout", "HEAD"))
			},
			expected: true,
		},
		{
			test: "#3 TestCase: Index Written But A File Is Missing (Interrupted Reset)",
			prepare: func(t *testing.T, f *fixture) {
				require.NoError(t, gitCmd("-C", f.dir, "checkout", "HEAD"))
				require.NoError(t, os.Remove(filepath.Join(f.dir, "chart.yaml")))
			},
			expected: false,
		},
		{
			test: "#4 TestCase: A File Has A Different Size",
			prepare: func(t *testing.T, f *fixture) {
				require.NoError(t, gitCmd("-C", f.dir, "checkout", "HEAD"))
				require.NoError(t, os.WriteFile(filepath.Join(f.dir, "chart.yaml"), []byte("truncated?"), 0o644))
			},
			expected: false,
		},
		{
			test: "#5 TestCase: Index Is At A Different Commit Than HEAD",
			prepare: func(t *testing.T, f *fixture) {
				require.NoError(t, gitCmd("-C", f.dir, "checkout", "HEAD"))
				require.NoError(t, os.WriteFile(filepath.Join(f.dir, "chart.yaml"), []byte("v2, longer"), 0o644))
				require.NoError(t, gitCmd("-C", f.dir, "-c", "user.name=rancher", "-c", "user.email=rancher@suse.com",
					"-c", "commit.gpgsign=false", "commit", "-am", "v2"))
				// HEAD goes back to v1 while the index and the files stay at v2
				require.NoError(t, gitCmd("-C", f.dir, "reset", "--soft", "HEAD~1"))
			},
			expected: false,
		},
		{
			test: "#6 TestCase: Files Present But No Index",
			prepare: func(t *testing.T, f *fixture) {
				require.NoError(t, gitCmd("-C", f.dir, "checkout", "HEAD"))
				require.NoError(t, os.Remove(filepath.Join(f.dir, ".git", "index")))
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			f := newFixture(t)
			tc.prepare(t, f)

			assert.Equal(t, tc.expected, worktreePopulated(f.dir))
			assert.Equal(t, tc.expected, markIfPopulated(f.dir))
			assert.Equal(t, tc.expected, IsCheckedOut(bundledName, bundledURL))
		})
	}
	t.Run("#7 TestCase: Directory Is Not A Repository", func(t *testing.T) {
		assert.False(t, worktreePopulated(t.TempDir()))
	})
}

// Test_Ensure_ShippedWithFiles checks a repository shipped with its files is reported ready by Ensure
// even when what it goes on to do can't succeed, as there is no upstream to fetch from.
func Test_Ensure_ShippedWithFiles(t *testing.T) {
	setSystemCatalog(t, "external")
	f := newFixture(t)
	require.NoError(t, gitCmd("-C", f.dir, "checkout", "HEAD"))
	require.False(t, IsCheckedOut(bundledName, bundledURL))

	err := Ensure(nil, bundledNamespace, bundledName, bundledURL, "missing-commit", false, nil)

	require.ErrorContains(t, err, "ensure failure")
	assert.True(t, IsCheckedOut(bundledName, bundledURL))
}
