package git

import (
	"errors"
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
