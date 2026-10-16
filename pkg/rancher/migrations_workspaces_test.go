package rancher

import (
	"errors"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	provisioningv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/pagination"
	"github.com/rancher/rancher/pkg/pagination/paginationtest"
	rancherversion "github.com/rancher/rancher/pkg/version"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// workspaceClusters returns a pager for management clusters with the given fleet workspace names.
func workspaceClusters(names []string, perPage int) *paginationtest.Pager[apiv3.Cluster] {
	var clusters []apiv3.Cluster
	for _, name := range names {
		clusters = append(clusters, apiv3.Cluster{Spec: apiv3.ClusterSpec{FleetWorkspaceName: name}})
	}
	return paginationtest.NewPager(clusters, perPage)
}

// workspaceClusterClient records the namespaces that the provisioning cluster list calls use.
type workspaceClusterClient struct {
	provisioningv1.ClusterController
	t          *testing.T
	namespaces []string
	results    map[string]error // error to return for a namespace
}

func (c *workspaceClusterClient) List(namespace string, opts metav1.ListOptions) (*provv1.ClusterList, error) {
	assert.Equal(c.t, pagination.PageSize, opts.Limit, "list calls must set the page size")
	c.namespaces = append(c.namespaces, namespace)
	if err := c.results[namespace]; err != nil {
		return nil, err
	}
	return &provv1.ClusterList{}, nil
}

type fakeProvisioningInterface struct {
	provisioningv1.Interface
	cluster provisioningv1.ClusterController
}

func (f *fakeProvisioningInterface) Cluster() provisioningv1.ClusterController {
	return f.cluster
}

func TestFleetWorkspaceNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		names         []string
		perPage       int
		failPage      int
		failErr       error
		want          []string
		wantErr       bool
		wantListCalls int
	}{
		{
			name:          "single page keeps list order and removes duplicates",
			names:         []string{"fleet-b", "fleet-a", "fleet-b"},
			perPage:       3,
			want:          []string{"fleet-b", "fleet-a"},
			wantListCalls: 1,
		},
		{
			name:          "duplicates across pages are removed",
			names:         []string{"fleet-a", "fleet-b", "fleet-b", "fleet-c", "fleet-a"},
			perPage:       2,
			want:          []string{"fleet-a", "fleet-b", "fleet-c"},
			wantListCalls: 3,
		},
		{
			name:          "empty name on a later page replaces all names",
			names:         []string{"fleet-a", "fleet-b", ""},
			perPage:       2,
			want:          []string{""},
			wantListCalls: 2,
		},
		{
			name:          "empty name on the first page replaces all names",
			names:         []string{"", "fleet-a", "fleet-b"},
			perPage:       2,
			want:          []string{""},
			wantListCalls: 2,
		},
		{
			name:          "no clusters returns no names",
			names:         nil,
			perPage:       2,
			want:          nil,
			wantListCalls: 1,
		},
		{
			name:          "expired continue token is returned without a resume",
			names:         []string{"fleet-a", "fleet-b"},
			perPage:       1,
			failPage:      2,
			failErr:       paginationtest.ExpiredError(),
			wantErr:       true,
			wantListCalls: 2,
		},
		{
			name:          "list error on page 2 is returned",
			names:         []string{"fleet-a", "fleet-b"},
			perPage:       1,
			failPage:      2,
			wantErr:       true,
			wantListCalls: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pager := workspaceClusters(tc.names, tc.perPage).FailOnPage(tc.failPage, tc.failErr)
			got, err := fleetWorkspaceNames(mgmtClusterMock(gomock.NewController(t), pager))
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, got)
			assert.Len(t, pager.Calls, tc.wantListCalls)
			pager.AssertLimit(t)
		})
	}
}

func TestMigrateMachinePoolsDynamicSchemaLabel_Workspaces(t *testing.T) {
	// This test changes the global version, so it must not run in parallel.
	originalVersion := rancherversion.Version
	rancherversion.Version = "v2.8.0"
	t.Cleanup(func() {
		rancherversion.Version = originalVersion
	})

	notFound := apierrors.NewNotFound(schema.GroupResource{Group: "provisioning.cattle.io", Resource: "clusters"}, "x")
	errBoom := errors.New("boom")

	tests := []struct {
		name           string
		names          []string
		perPage        int
		provResults    map[string]error
		wantNamespaces []string
		wantErr        error
		wantMigrated   bool
	}{
		{
			name:           "each workspace is listed once across pages",
			names:          []string{"fleet-a", "fleet-b", "fleet-a", "fleet-c"},
			perPage:        2,
			wantNamespaces: []string{"fleet-a", "fleet-b", "fleet-c"},
			wantMigrated:   true,
		},
		{
			name:           "empty workspace lists all namespaces once",
			names:          []string{"fleet-a", "", "fleet-b"},
			perPage:        1,
			wantNamespaces: []string{""},
			wantMigrated:   true,
		},
		{
			name:           "NotFound from the cluster list skips the workspace",
			names:          []string{"fleet-a", "fleet-b"},
			perPage:        2,
			provResults:    map[string]error{"fleet-a": notFound},
			wantNamespaces: []string{"fleet-a", "fleet-b"},
			wantMigrated:   true,
		},
		{
			name:           "other cluster list error stops the migration",
			names:          []string{"fleet-a", "fleet-b"},
			perPage:        2,
			provResults:    map[string]error{"fleet-a": errBoom},
			wantNamespaces: []string{"fleet-a"},
			wantErr:        errBoom,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mgmt := mgmtClusterMock(gomock.NewController(t), workspaceClusters(tc.names, tc.perPage))
			prov := &workspaceClusterClient{t: t, results: tc.provResults}
			cmClient := &fakeMigrationConfigMapClient{}

			w := &wrangler.CAPIContext{
				Context: &wrangler.Context{
					Mgmt:         &migMgmt{cluster: mgmt},
					Provisioning: &fakeProvisioningInterface{cluster: prov},
					Core:         &migCore{cm: cmClient},
				},
			}

			err := migrateMachinePoolsDynamicSchemaLabel(w)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantNamespaces, prov.namespaces)
			if tc.wantMigrated {
				require.NotNil(t, cmClient.cm)
				assert.Equal(t, "true", cmClient.cm.Data[dynamicSchemaMachinePoolsMigratedKey])
			} else {
				assert.Nil(t, cmClient.cm, "the migration must not be marked as done")
			}
		})
	}
}
