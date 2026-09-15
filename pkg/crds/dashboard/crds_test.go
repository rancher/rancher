package dashboard

import (
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/rancher/pkg/crds"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

func TestList(t *testing.T) {
	defer setupListTestState(false, false)()

	result, err := List(nil)
	require.NoError(t, err, "unexpected error while listing CRDs")
	require.Truef(t, hasCRD(result, "clusters.management.cattle.io"), "missing expected clusters CRD result=%v", result)

	// test that when the CRD is in the migrated list it does not get installed.
	crds.MigratedResources = map[string]bool{"clusters.management.cattle.io": true}

	result, err = List(nil)
	require.NoError(t, err, "unexpected error while listing CRDs")
	require.False(t, hasCRD(result, "clusters.management.cattle.io"), "unexpected clusters CRD result")
}

func TestListSkipsPlanCRDWhenItAlreadyExists(t *testing.T) {
	defer setupListTestState(false, false)()

	server := newFakeCRDServer(withCRDExists(map[string]bool{
		"plans.upgrade.cattle.io": true,
	}))
	defer server.Close()

	result, err := List(&rest.Config{Host: server.URL})
	require.NoError(t, err, "unexpected error while listing CRDs")
	require.False(t, hasCRD(result, "plans.upgrade.cattle.io"), "unexpected plan CRD result when CRD already exists")
}

func TestListWithFleetEnabledIncludesFleetBootstrapCRDs(t *testing.T) {
	defer setupListTestState(true, false)()

	server := newFakeCRDServer(withCRDStatus(map[string]int{
		"plans.upgrade.cattle.io": http.StatusOK,
		// Fleet CRDs do not exist yet, so List should include bootstrap entries for them.
		"bundles.fleet.cattle.io":       http.StatusNotFound,
		"clusters.fleet.cattle.io":      http.StatusNotFound,
		"clustergroups.fleet.cattle.io": http.StatusNotFound,
		"helmops.fleet.cattle.io":       http.StatusNotFound,
	}))
	defer server.Close()

	result, err := List(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	requireCRDsPresent(t, result,
		"fleetworkspaces.management.cattle.io",
		"bundles.fleet.cattle.io",
		"clusters.fleet.cattle.io",
		"clustergroups.fleet.cattle.io",
		"helmops.fleet.cattle.io",
	)
}

func TestListWithFleetAndProvisioningV2IncludesManagedChartCRD(t *testing.T) {
	defer setupListTestState(true, true)()

	server := newFakeCRDServer(withCRDStatus(map[string]int{
		"plans.upgrade.cattle.io":       http.StatusOK,
		"bundles.fleet.cattle.io":       http.StatusOK,
		"clusters.fleet.cattle.io":      http.StatusOK,
		"clustergroups.fleet.cattle.io": http.StatusOK,
		"helmops.fleet.cattle.io":       http.StatusOK,
	}))
	defer server.Close()

	result, err := List(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	requireCRDsPresent(t, result,
		"managedcharts.management.cattle.io",
		"fleetworkspaces.management.cattle.io",
	)
}

func TestListReturnsErrorWhenPlanBootstrapFails(t *testing.T) {
	defer setupListTestState(false, false)()

	result, err := List(&rest.Config{Host: "invalid://bad"})
	require.Error(t, err)
	require.Nil(t, result)
}

func TestListReturnsErrorWhenFleetBootstrapFails(t *testing.T) {
	defer setupListTestState(true, false)()

	server := newFakeCRDServer(withCRDStatus(map[string]int{
		"plans.upgrade.cattle.io": http.StatusOK,
		"bundles.fleet.cattle.io": http.StatusInternalServerError,
	}))
	defer server.Close()

	result, err := List(&rest.Config{Host: server.URL})
	require.Error(t, err)
	require.Nil(t, result)
}

func TestPlanBootstrap(t *testing.T) {
	base := []crd.CRD{{GVK: schema.GroupVersionKind{Group: "test.cattle.io", Version: "v1", Kind: "Foo"}}}

	t.Run("nil config appends plan crd", func(t *testing.T) {
		result, err := planBootstrap(base, nil)
		require.NoError(t, err)
		require.True(t, hasCRD(result, "plans.upgrade.cattle.io"))
	})

	t.Run("existing plan crd does not append", func(t *testing.T) {
		server := newFakeCRDServer(withCRDStatus(map[string]int{"plans.upgrade.cattle.io": http.StatusOK}))
		defer server.Close()

		result, err := planBootstrap(base, &rest.Config{Host: server.URL})
		require.NoError(t, err)
		require.False(t, hasCRD(result, "plans.upgrade.cattle.io"))
		require.Equal(t, len(base), len(result))
	})

	t.Run("missing plan crd appends", func(t *testing.T) {
		server := newFakeCRDServer(withCRDStatus(map[string]int{"plans.upgrade.cattle.io": http.StatusNotFound}))
		defer server.Close()

		result, err := planBootstrap(base, &rest.Config{Host: server.URL})
		require.NoError(t, err)
		require.True(t, hasCRD(result, "plans.upgrade.cattle.io"))
	})

	t.Run("api error returns error", func(t *testing.T) {
		server := newFakeCRDServer(withCRDStatus(map[string]int{"plans.upgrade.cattle.io": http.StatusInternalServerError}))
		defer server.Close()

		result, err := planBootstrap(base, &rest.Config{Host: server.URL})
		require.Error(t, err)
		require.Nil(t, result)
	})
}

func newFakeCRDServer(options ...fakeCRDServerOption) *httptest.Server {
	cfg := fakeCRDServerConfig{
		statusByName: map[string]int{},
	}
	for _, option := range options {
		option(&cfg)
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/apis/apiextensions.k8s.io/v1/customresourcedefinitions/"
		if len(r.URL.Path) >= len(prefix) && r.URL.Path[:len(prefix)] == prefix {
			name := r.URL.Path[len(prefix):]
			status, ok := cfg.statusByName[name]
			if !ok {
				status = http.StatusNotFound
			}

			switch status {
			case http.StatusOK:
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition","metadata":{"name":%q}}`, name)
				return
			case http.StatusNotFound:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprintf(w, `{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"customresourcedefinitions.apiextensions.k8s.io %q not found","reason":"NotFound","details":{"name":%q,"group":"apiextensions.k8s.io","kind":"customresourcedefinitions"},"code":404}`, name, name)
				return
			default:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = fmt.Fprintf(w, `{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"server error","reason":"InternalError","code":%d}`, status)
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
	}))
}

type fakeCRDServerConfig struct {
	statusByName map[string]int
}

type fakeCRDServerOption func(*fakeCRDServerConfig)

func withCRDExists(existing map[string]bool) fakeCRDServerOption {
	return func(cfg *fakeCRDServerConfig) {
		for name, exists := range existing {
			if exists {
				cfg.statusByName[name] = http.StatusOK
				continue
			}
			cfg.statusByName[name] = http.StatusNotFound
		}
	}
}

func withCRDStatus(statusByName map[string]int) fakeCRDServerOption {
	return func(cfg *fakeCRDServerConfig) {
		maps.Copy(cfg.statusByName, statusByName)
	}
}

func hasCRD(crds []crd.CRD, name string) bool {
	for _, c := range crds {
		if c.Name() == name {
			return true
		}
	}

	return false
}

func requireCRDsPresent(t *testing.T, crds []crd.CRD, names ...string) {
	t.Helper()
	for _, name := range names {
		require.Truef(t, hasCRD(crds, name), "missing expected CRD %s", name)
	}
}

func setFeatureForTest(feature *features.Feature, value bool) func() {
	original := feature.Enabled()
	feature.Set(value)

	return func() {
		feature.Set(original)
	}
}

func setupListTestState(fleetEnabled, provisioningV2Enabled bool) func() {
	restoreFleet := setFeatureForTest(features.Fleet, fleetEnabled)
	restoreProvisioning := setFeatureForTest(features.ProvisioningV2, provisioningV2Enabled)
	originalMigrated := crds.MigratedResources
	crds.MigratedResources = nil

	return func() {
		crds.MigratedResources = originalMigrated
		restoreProvisioning()
		restoreFleet()
	}
}
