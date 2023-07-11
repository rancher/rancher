package dashboard

import (
	"testing"

	"github.com/rancher/rancher/pkg/crds"
	"github.com/rancher/rancher/pkg/features"
	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
	// setting fleet to false so we can test with a nil cfg
	features.Fleet.Set(false)

	originalMigrated := crds.MigratedResources
	defer func() {
		crds.MigratedResources = originalMigrated
	}()

	crds.MigratedResources = nil

	result, err := List(nil)
	require.NoError(t, err, "unexpected error while listing CRDs")
	found := false
	for _, crd := range result {
		if crd.Name() == "clusters.management.cattle.io" {
			found = true
			break
		}
	}
	require.Truef(t, found, "missing expected clusters CRD result=%v", result)

	// test that when the CRD is in the migrated list it does not get installed.
	crds.MigratedResources = map[string]bool{"clusters.management.cattle.io": true}

	result, err = List(nil)
	require.NoError(t, err, "unexpected error while listing CRDs")

	for _, crd := range result {
		if crd.Name() == "clusters.management.cattle.io" {
			require.FailNow(t, "clusters.management.cattle.io", "unexpected clusters CRD result")
		}
	}
}
