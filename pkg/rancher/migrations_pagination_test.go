package rancher

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta2"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/pagination/paginationtest"
	rancherversion "github.com/rancher/rancher/pkg/version"
	"github.com/rancher/rancher/pkg/wrangler"
	controllerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The tests in this file change the global Rancher version, so none of them run in parallel.

const (
	migTestWorkspace = "fleet-default"
	// migMachineLabel has a prefix that the migration copies.
	migMachineLabel = "rke.cattle.io/test-label"
)

var (
	errMigBoom     = errors.New("boom")
	errMigNotFound = apierrors.NewNotFound(schema.GroupResource{Resource: "things"}, "x")
)

func clusterListOf(items []apiv3.Cluster, next string) *apiv3.ClusterList {
	return &apiv3.ClusterList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}
}

func provListOf(items []provv1.Cluster, next string) *provv1.ClusterList {
	return &provv1.ClusterList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}
}

// Interface wrappers that expose only the controllers that the migrations use.

type migMgmt struct {
	managementv3.Interface
	cluster managementv3.ClusterController
	crt     managementv3.ClusterRegistrationTokenController
	node    managementv3.NodeController
	schema  managementv3.DynamicSchemaController
}

func (m *migMgmt) Cluster() managementv3.ClusterController { return m.cluster }
func (m *migMgmt) ClusterRegistrationToken() managementv3.ClusterRegistrationTokenController {
	return m.crt
}
func (m *migMgmt) Node() managementv3.NodeController                   { return m.node }
func (m *migMgmt) DynamicSchema() managementv3.DynamicSchemaController { return m.schema }

type migCore struct {
	controllerv1.Interface
	cm     controllerv1.ConfigMapController
	secret controllerv1.SecretController
}

func (c *migCore) ConfigMap() controllerv1.ConfigMapController { return c.cm }
func (c *migCore) Secret() controllerv1.SecretController       { return c.secret }

type migRKE struct {
	rkecontrollers.Interface
	controlPlane rkecontrollers.RKEControlPlaneController
	bootstrap    rkecontrollers.RKEBootstrapController
}

func (r *migRKE) RKEControlPlane() rkecontrollers.RKEControlPlaneController { return r.controlPlane }
func (r *migRKE) RKEBootstrap() rkecontrollers.RKEBootstrapController       { return r.bootstrap }

type migCAPI struct {
	capicontrollers.Interface
	machine capicontrollers.MachineController
}

func (c *migCAPI) Machine() capicontrollers.MachineController { return c.machine }

// migProvisioning reuses the provisioning wrapper of the workspace tests.
func migProvisioning(c provisioningv1.ClusterController) provisioningv1.Interface {
	return &fakeProvisioningInterface{cluster: c}
}

// migSetup sets a version that enables the migrations and returns a gomock controller.
func migSetup(t *testing.T) *gomock.Controller {
	t.Helper()
	original := rancherversion.Version
	rancherversion.Version = "v2.8.0"
	t.Cleanup(func() { rancherversion.Version = original })
	return gomock.NewController(t)
}

// migConfigMap returns a config map client. If migrated is true, the migration key is already set.
func migConfigMap(migrated bool, key string) *fakeMigrationConfigMapClient {
	c := &fakeMigrationConfigMapClient{}
	if migrated {
		c.cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: cattleNamespace, Name: "migration", ResourceVersion: "1"},
			Data:       map[string]string{key: "true"},
		}
	}
	return c
}

func isMigrated(c *fakeMigrationConfigMapClient, key string) bool {
	return c.cm != nil && c.cm.Data[key] == "true"
}

// mgmtClusterMock returns a non-namespaced mock that serves the pager on List.
func mgmtClusterMock(ctrl *gomock.Controller, p *paginationtest.Pager[apiv3.Cluster]) *fake.MockNonNamespacedControllerInterface[*apiv3.Cluster, *apiv3.ClusterList] {
	m := fake.NewMockNonNamespacedControllerInterface[*apiv3.Cluster, *apiv3.ClusterList](ctrl)
	m.EXPECT().List(gomock.Any()).DoAndReturn(func(opts metav1.ListOptions) (*apiv3.ClusterList, error) {
		items, next, err := p.Serve(opts)
		if err != nil {
			return nil, err
		}
		return clusterListOf(items, next), nil
	}).AnyTimes()
	return m
}

// provClusterMock returns a mock that serves the pager on List.
func provClusterMock(ctrl *gomock.Controller, p *paginationtest.Pager[provv1.Cluster]) *fake.MockControllerInterface[*provv1.Cluster, *provv1.ClusterList] {
	m := fake.NewMockControllerInterface[*provv1.Cluster, *provv1.ClusterList](ctrl)
	m.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, opts metav1.ListOptions) (*provv1.ClusterList, error) {
		items, next, err := p.Serve(opts)
		if err != nil {
			return nil, err
		}
		return provListOf(items, next), nil
	}).AnyTimes()
	return m
}

func names[T any](items []T, name func(T) string) []string {
	var out []string
	for _, i := range items {
		out = append(out, name(i))
	}
	return out
}

func TestMigrateEncryptionKeyRotationLeader_Pagination(t *testing.T) {
	clusters := []apiv3.Cluster{
		{ObjectMeta: metav1.ObjectMeta{Name: "c1"}, Spec: apiv3.ClusterSpec{FleetWorkspaceName: migTestWorkspace}},
		{ObjectMeta: metav1.ObjectMeta{Name: "c2"}, Spec: apiv3.ClusterSpec{FleetWorkspaceName: migTestWorkspace}},
		{ObjectMeta: metav1.ObjectMeta{Name: "c3"}, Spec: apiv3.ClusterSpec{FleetWorkspaceName: migTestWorkspace}},
	}
	const annotation = "rke.cattle.io/encrypt-key-rotation-leader"

	tests := []struct {
		name         string
		migrated     bool
		failPage     int
		leaders      map[string]string // cluster name -> leader annotation
		getErrs      map[string]error
		wantErr      error
		wantTokens   []string
		wantStatuses []string
		wantMigrated bool
	}{
		{
			name:         "leaders on every page move to the status",
			leaders:      map[string]string{"c1": "leader-1", "c3": "leader-3"},
			wantTokens:   []string{"", "token-page-1"},
			wantStatuses: []string{"c1:leader-1", "c3:leader-3"},
			wantMigrated: true,
		},
		{
			name:         "missing control plane is skipped",
			leaders:      map[string]string{"c1": "leader-1", "c3": "leader-3"},
			getErrs:      map[string]error{"c1": errMigNotFound},
			wantTokens:   []string{"", "token-page-1"},
			wantStatuses: []string{"c3:leader-3"},
			wantMigrated: true,
		},
		{
			name:       "control plane error stops the migration",
			getErrs:    map[string]error{"c2": errMigBoom},
			wantErr:    errMigBoom,
			wantTokens: []string{""},
		},
		{
			name:         "list error on page 2 keeps the page 1 work and does not mark the migration",
			failPage:     2,
			leaders:      map[string]string{"c1": "leader-1", "c3": "leader-3"},
			wantTokens:   []string{"", "token-page-1"},
			wantStatuses: []string{"c1:leader-1"},
		},
		{
			name:     "migrated flag skips all list calls",
			migrated: true,
			leaders:  map[string]string{"c1": "leader-1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := migSetup(t)
			pager := paginationtest.NewPager(clusters, 2).FailOnPage(tc.failPage, nil)

			var statuses, updates []string
			cp := fake.NewMockControllerInterface[*rkev1.RKEControlPlane, *rkev1.RKEControlPlaneList](ctrl)
			cp.EXPECT().Get(migTestWorkspace, gomock.Any(), gomock.Any()).DoAndReturn(func(_, name string, _ metav1.GetOptions) (*rkev1.RKEControlPlane, error) {
				if err := tc.getErrs[name]; err != nil {
					return nil, err
				}
				obj := &rkev1.RKEControlPlane{ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: map[string]string{}}}
				if leader := tc.leaders[name]; leader != "" {
					obj.Annotations[annotation] = leader
				}
				return obj, nil
			}).AnyTimes()
			cp.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(obj *rkev1.RKEControlPlane) (*rkev1.RKEControlPlane, error) {
				statuses = append(statuses, obj.Name+":"+obj.Status.RotateEncryptionKeysLeader)
				return obj, nil
			}).AnyTimes()
			cp.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *rkev1.RKEControlPlane) (*rkev1.RKEControlPlane, error) {
				_, hasAnnotation := obj.Annotations[annotation]
				assert.False(t, hasAnnotation, "the leader annotation must be removed")
				updates = append(updates, obj.Name)
				return obj, nil
			}).AnyTimes()

			cmClient := migConfigMap(tc.migrated, encryptionKeyRotationStatusMigratedKey)
			w := &wrangler.CAPIContext{Context: &wrangler.Context{
				Mgmt: &migMgmt{cluster: mgmtClusterMock(ctrl, pager)},
				Core: &migCore{cm: cmClient},
				RKE:  &migRKE{controlPlane: cp},
			}}

			err := migrateEncryptionKeyRotationLeader(w)
			switch {
			case tc.failPage > 0:
				require.ErrorContains(t, err, "simulated list error")
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
			default:
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantTokens, pager.Tokens())
			pager.AssertLimit(t)
			assert.Equal(t, tc.wantStatuses, statuses)
			assert.Len(t, updates, len(tc.wantStatuses))
			assert.Equal(t, tc.wantMigrated, isMigrated(cmClient, encryptionKeyRotationStatusMigratedKey) && !tc.migrated)
		})
	}
}

func TestMigrateSystemAgentDataDirectory_Pagination(t *testing.T) {
	envVar := func(name, value string) rkev1.EnvVar { return rkev1.EnvVar{Name: name, Value: value} }
	newCluster := func(name string, vars ...rkev1.EnvVar) provv1.Cluster {
		return provv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: migTestWorkspace},
			Spec:       provv1.ClusterSpec{AgentEnvVars: vars, RKEConfig: &provv1.RKEConfig{}},
		}
	}
	clusters := []provv1.Cluster{
		newCluster("c1", envVar("X", "1"), envVar(capr.SystemAgentDataDirEnvVar, "/data/a")),
		newCluster("c2", envVar("Y", "2")),
		newCluster("c3", envVar(capr.SystemAgentDataDirEnvVar, "/old"), envVar(capr.SystemAgentDataDirEnvVar, "/new")),
	}

	tests := []struct {
		name         string
		failPage     int
		updateErrs   map[string]error
		wantErr      string
		wantTokens   []string
		wantUpdated  []string
		wantMigrated bool
	}{
		{
			name:         "clusters on every page move the data directory",
			wantTokens:   []string{"", "token-page-1"},
			wantUpdated:  []string{"c1:/data/a:1", "c3:/new:0"},
			wantMigrated: true,
		},
		{
			name:         "NotFound on update is tolerated",
			updateErrs:   map[string]error{"c1": errMigNotFound},
			wantTokens:   []string{"", "token-page-1"},
			wantUpdated:  []string{"c1:/data/a:1", "c3:/new:0"},
			wantMigrated: true,
		},
		{
			name:        "other update error stops the migration",
			updateErrs:  map[string]error{"c1": errMigBoom},
			wantErr:     "boom",
			wantTokens:  []string{""},
			wantUpdated: []string{"c1:/data/a:1"},
		},
		{
			name:        "list error on page 2 does not mark the migration",
			failPage:    2,
			wantErr:     "simulated list error",
			wantTokens:  []string{"", "token-page-1"},
			wantUpdated: []string{"c1:/data/a:1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := migSetup(t)
			pager := paginationtest.NewPager(clusters, 2).FailOnPage(tc.failPage, nil)
			prov := provClusterMock(ctrl, pager)

			var updated []string
			prov.EXPECT().Update(gomock.Any()).DoAndReturn(func(c *provv1.Cluster) (*provv1.Cluster, error) {
				updated = append(updated, fmt.Sprintf("%s:%s:%d", c.Name, c.Spec.RKEConfig.DataDirectories.SystemAgent, len(c.Spec.AgentEnvVars)))
				return c, tc.updateErrs[c.Name]
			}).AnyTimes()

			cmClient := migConfigMap(false, systemAgentVarDirMigratedKey)
			w := &wrangler.CAPIContext{Context: &wrangler.Context{
				Provisioning: migProvisioning(prov),
				Core:         &migCore{cm: cmClient},
			}}

			err := migrateSystemAgentDataDirectory(w)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantTokens, pager.Tokens())
			pager.AssertLimit(t)
			assert.Equal(t, tc.wantUpdated, updated)
			assert.Equal(t, tc.wantMigrated, isMigrated(cmClient, systemAgentVarDirMigratedKey))
		})
	}
}

func TestMigrateImportedClusterFields_Pagination(t *testing.T) {
	imported := func(name, mgmtName string) provv1.Cluster {
		return provv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: migTestWorkspace},
			Status:     provv1.ClusterStatus{ClusterName: mgmtName},
		}
	}
	clusters := []provv1.Cluster{
		imported("imported-1", "c-aaaaa"),
		imported("c-bbbbb", "c-bbbbb"), // legacy name, skipped
		{ObjectMeta: metav1.ObjectMeta{Name: "rke-1"}, Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{}}, Status: provv1.ClusterStatus{ClusterName: "c-ddddd"}}, // RKE, skipped
		imported("imported-2", "c-ccccc"),
	}

	tests := []struct {
		name         string
		failPage     int
		getErrs      map[string]error
		updateErrs   map[string]error
		wantErr      string
		wantTokens   []string
		wantGets     []string
		wantUpdated  []string
		wantMigrated bool
	}{
		{
			name:         "imported clusters on every page are updated",
			wantTokens:   []string{"", "token-page-1"},
			wantGets:     []string{"c-aaaaa", "c-ccccc"},
			wantUpdated:  []string{"c-aaaaa", "c-ccccc"},
			wantMigrated: true,
		},
		{
			name:         "missing management cluster is skipped",
			getErrs:      map[string]error{"c-aaaaa": errMigNotFound},
			wantTokens:   []string{"", "token-page-1"},
			wantGets:     []string{"c-aaaaa", "c-ccccc"},
			wantUpdated:  []string{"c-ccccc"},
			wantMigrated: true,
		},
		{
			name:         "NotFound on update is tolerated",
			updateErrs:   map[string]error{"c-aaaaa": errMigNotFound},
			wantTokens:   []string{"", "token-page-1"},
			wantGets:     []string{"c-aaaaa", "c-ccccc"},
			wantUpdated:  []string{"c-aaaaa", "c-ccccc"},
			wantMigrated: true,
		},
		{
			name:       "get error stops the migration",
			getErrs:    map[string]error{"c-aaaaa": errMigBoom},
			wantErr:    "boom",
			wantTokens: []string{""},
			wantGets:   []string{"c-aaaaa"},
		},
		{
			name:        "list error on page 2 does not mark the migration",
			failPage:    2,
			wantErr:     "simulated list error",
			wantTokens:  []string{"", "token-page-1"},
			wantGets:    []string{"c-aaaaa"},
			wantUpdated: []string{"c-aaaaa"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := migSetup(t)
			pager := paginationtest.NewPager(clusters, 2).FailOnPage(tc.failPage, nil)
			prov := provClusterMock(ctrl, pager)

			var gets, updated []string
			mgmtCluster := fake.NewMockNonNamespacedControllerInterface[*apiv3.Cluster, *apiv3.ClusterList](ctrl)
			mgmtCluster.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, _ metav1.GetOptions) (*apiv3.Cluster, error) {
				gets = append(gets, name)
				if err := tc.getErrs[name]; err != nil {
					return nil, err
				}
				c := &apiv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name}}
				c.Spec.DesiredAgentImage = "agent"
				c.Spec.DesiredAuthImage = "auth"
				c.Spec.ClusterSecrets.PrivateRegistryURL = "registry"
				return c, nil
			}).AnyTimes()
			mgmtCluster.EXPECT().Update(gomock.Any()).DoAndReturn(func(c *apiv3.Cluster) (*apiv3.Cluster, error) {
				assert.Empty(t, c.Spec.DesiredAgentImage)
				assert.Empty(t, c.Spec.DesiredAuthImage)
				assert.Empty(t, c.Spec.ClusterSecrets.PrivateRegistryURL)
				updated = append(updated, c.Name)
				return c, tc.updateErrs[c.Name]
			}).AnyTimes()

			cmClient := migConfigMap(false, importedClusterManagedFieldsMigratedKey)
			w := &wrangler.Context{
				Provisioning: migProvisioning(prov),
				Mgmt:         &migMgmt{cluster: mgmtCluster},
				Core:         &migCore{cm: cmClient},
			}

			err := migrateImportedClusterFields(w)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantTokens, pager.Tokens())
			pager.AssertLimit(t)
			assert.Equal(t, tc.wantGets, gets)
			assert.Equal(t, tc.wantUpdated, updated)
			assert.Equal(t, tc.wantMigrated, isMigrated(cmClient, importedClusterManagedFieldsMigratedKey))
		})
	}
}

func TestManagementNodeCleanup_Pagination(t *testing.T) {
	const finalizer = "controller.cattle.io/node-controller"
	nodes := []apiv3.Node{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "n1", Finalizers: []string{finalizer}}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "n2"}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "n3", Finalizers: []string{finalizer}}},
	}

	tests := []struct {
		name         string
		migrated     bool
		failPage     int
		getErrs      map[string]error
		wantErr      string
		wantTokens   []string
		wantUpdated  []string
		wantMigrated bool
	}{
		{
			name:     "migrated flag skips all list calls",
			migrated: true,
		},
		{
			name:         "nodes on every page lose the finalizer",
			wantTokens:   []string{"", "token-page-1"},
			wantUpdated:  []string{"n1", "n3"},
			wantMigrated: true,
		},
		{
			name:       "node error stops the migration",
			getErrs:    map[string]error{"n2": errMigBoom},
			wantErr:    "boom",
			wantTokens: []string{""},
			// n1 is cleaned before n2 fails.
			wantUpdated: []string{"n1"},
		},
		{
			name:        "list error on page 2 does not mark the migration",
			failPage:    2,
			wantErr:     "simulated list error",
			wantTokens:  []string{"", "token-page-1"},
			wantUpdated: []string{"n1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := migSetup(t)
			pager := paginationtest.NewPager(nodes, 2).FailOnPage(tc.failPage, nil)

			var updated []string
			nodeCtrl := fake.NewMockControllerInterface[*apiv3.Node, *apiv3.NodeList](ctrl)
			nodeCtrl.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, opts metav1.ListOptions) (*apiv3.NodeList, error) {
				items, next, err := pager.Serve(opts)
				if err != nil {
					return nil, err
				}
				return &apiv3.NodeList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}, nil
			}).AnyTimes()
			nodeCtrl.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ns, name string, _ metav1.GetOptions) (*apiv3.Node, error) {
				if err := tc.getErrs[name]; err != nil {
					return nil, err
				}
				for i := range nodes {
					if nodes[i].Name == name {
						return nodes[i].DeepCopy(), nil
					}
				}
				return nil, errMigNotFound
			}).AnyTimes()
			nodeCtrl.EXPECT().Update(gomock.Any()).DoAndReturn(func(n *apiv3.Node) (*apiv3.Node, error) {
				assert.NotContains(t, n.Finalizers, finalizer)
				updated = append(updated, n.Name)
				return n, nil
			}).AnyTimes()

			cmClient := migConfigMap(tc.migrated, managementCleanupCompletedKey)
			w := &wrangler.Context{Mgmt: &migMgmt{node: nodeCtrl}, Core: &migCore{cm: cmClient}}

			err := managementNodeCleanup(w)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantTokens, pager.Tokens())
			pager.AssertLimit(t)
			assert.Equal(t, tc.wantUpdated, updated)
			assert.Equal(t, tc.wantMigrated, isMigrated(cmClient, managementCleanupCompletedKey) && !tc.migrated)
		})
	}
}

func TestCleanupMgmtOnlyConditionsFromProvisioningClusters_Pagination(t *testing.T) {
	cond := func(types ...string) []genericcondition.GenericCondition {
		var out []genericcondition.GenericCondition
		for _, typ := range types {
			out = append(out, genericcondition.GenericCondition{Type: typ})
		}
		return out
	}
	waiting := string(apiv3.ClusterConditionWaiting)
	newCluster := func(name string, types ...string) provv1.Cluster {
		return provv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: migTestWorkspace},
			Status:     provv1.ClusterStatus{Conditions: cond(types...)},
		}
	}
	clusters := []provv1.Cluster{
		newCluster("a", waiting, "Ready"),
		newCluster("b"),
		newCluster("c", "Ready"),
		newCluster("d", waiting),
	}

	tests := []struct {
		name         string
		failPage     int
		getErrs      map[string]error
		statusErrs   map[string]error
		wantErr      string
		wantTokens   []string
		wantGets     []string
		wantUpdated  []string
		wantMigrated bool
	}{
		{
			name:         "conditions on every page are cleaned",
			wantTokens:   []string{"", "token-page-1"},
			wantGets:     []string{"a", "c", "d"},
			wantUpdated:  []string{"a:Ready", "d:"},
			wantMigrated: true,
		},
		{
			name:         "missing cluster is skipped",
			getErrs:      map[string]error{"a": errMigNotFound},
			wantTokens:   []string{"", "token-page-1"},
			wantGets:     []string{"a", "c", "d"},
			wantUpdated:  []string{"d:"},
			wantMigrated: true,
		},
		{
			name:         "NotFound on status update is tolerated",
			statusErrs:   map[string]error{"a": errMigNotFound},
			wantTokens:   []string{"", "token-page-1"},
			wantGets:     []string{"a", "c", "d"},
			wantUpdated:  []string{"a:Ready", "d:"},
			wantMigrated: true,
		},
		{
			name:        "status update error stops the migration",
			statusErrs:  map[string]error{"a": errMigBoom},
			wantErr:     "boom",
			wantTokens:  []string{""},
			wantGets:    []string{"a"},
			wantUpdated: []string{"a:Ready"},
		},
		{
			name:        "list error on page 2 does not mark the migration",
			failPage:    2,
			wantErr:     "simulated list error",
			wantTokens:  []string{"", "token-page-1"},
			wantGets:    []string{"a"},
			wantUpdated: []string{"a:Ready"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := migSetup(t)
			pager := paginationtest.NewPager(clusters, 2).FailOnPage(tc.failPage, nil)
			prov := provClusterMock(ctrl, pager)

			var gets, updated []string
			prov.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_, name string, _ metav1.GetOptions) (*provv1.Cluster, error) {
				gets = append(gets, name)
				if err := tc.getErrs[name]; err != nil {
					return nil, err
				}
				for i := range clusters {
					if clusters[i].Name == name {
						return clusters[i].DeepCopy(), nil
					}
				}
				return nil, errMigNotFound
			}).AnyTimes()
			prov.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(c *provv1.Cluster) (*provv1.Cluster, error) {
				var left string
				for _, cnd := range c.Status.Conditions {
					left += cnd.Type
				}
				updated = append(updated, c.Name+":"+left)
				return c, tc.statusErrs[c.Name]
			}).AnyTimes()

			cmClient := migConfigMap(false, provisioningClusterMgmtOnlyConditionsCleanedUpKey)
			w := &wrangler.CAPIContext{Context: &wrangler.Context{
				Provisioning: migProvisioning(prov),
				Core:         &migCore{cm: cmClient},
			}}

			err := cleanupMgmtOnlyConditionsFromProvisioningClusters(w)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantTokens, pager.Tokens())
			pager.AssertLimit(t)
			assert.Equal(t, tc.wantGets, gets)
			assert.Equal(t, tc.wantUpdated, updated)
			assert.Equal(t, tc.wantMigrated, isMigrated(cmClient, provisioningClusterMgmtOnlyConditionsCleanedUpKey))
		})
	}
}

func TestMigrateExistingRKE2ClustersPrimeAnnotation_Pagination(t *testing.T) {
	newCluster := func(name, version string, rkeConfig bool, annotations map[string]string) provv1.Cluster {
		c := provv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: migTestWorkspace, Annotations: annotations},
			Spec:       provv1.ClusterSpec{KubernetesVersion: version},
		}
		if rkeConfig {
			c.Spec.RKEConfig = &provv1.RKEConfig{}
		}
		return c
	}
	clusters := []provv1.Cluster{
		newCluster("rke2-a", "v1.30.0+rke2r1", true, nil),
		newCluster("k3s", "v1.30.0+k3s1", true, nil),
		newCluster("imported", "", false, nil),
		newCluster("rke2-annotated", "v1.30.0+rke2r1", true, map[string]string{capr.RKE2PrimeEnabledAnnotation: "true"}),
		newCluster("rke2-b", "v1.30.0+rke2r1", true, nil),
	}

	tests := []struct {
		name         string
		failPage     int
		getErrs      map[string]error
		updateErrs   map[string]error
		wantErr      string
		wantTokens   []string
		wantUpdated  []string
		wantMigrated bool
	}{
		{
			name:         "only RKE2 clusters without the annotation are updated on every page",
			wantTokens:   []string{"", "token-page-1", "token-page-2"},
			wantUpdated:  []string{"rke2-a", "rke2-b"},
			wantMigrated: true,
		},
		{
			name:         "missing cluster is skipped",
			getErrs:      map[string]error{"rke2-a": errMigNotFound},
			wantTokens:   []string{"", "token-page-1", "token-page-2"},
			wantUpdated:  []string{"rke2-b"},
			wantMigrated: true,
		},
		{
			name:         "NotFound on update is tolerated",
			updateErrs:   map[string]error{"rke2-a": errMigNotFound},
			wantTokens:   []string{"", "token-page-1", "token-page-2"},
			wantUpdated:  []string{"rke2-a", "rke2-b"},
			wantMigrated: true,
		},
		{
			name:        "update error stops the migration",
			updateErrs:  map[string]error{"rke2-a": errMigBoom},
			wantErr:     "boom",
			wantTokens:  []string{""},
			wantUpdated: []string{"rke2-a"},
		},
		{
			name:        "list error on page 3 does not mark the migration",
			failPage:    3,
			wantErr:     "simulated list error",
			wantTokens:  []string{"", "token-page-1", "token-page-2"},
			wantUpdated: []string{"rke2-a"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := migSetup(t)
			pager := paginationtest.NewPager(clusters, 2).FailOnPage(tc.failPage, nil)
			prov := provClusterMock(ctrl, pager)

			var updated []string
			prov.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_, name string, _ metav1.GetOptions) (*provv1.Cluster, error) {
				if err := tc.getErrs[name]; err != nil {
					return nil, err
				}
				for i := range clusters {
					if clusters[i].Name == name {
						return clusters[i].DeepCopy(), nil
					}
				}
				return nil, errMigNotFound
			}).AnyTimes()
			prov.EXPECT().Update(gomock.Any()).DoAndReturn(func(c *provv1.Cluster) (*provv1.Cluster, error) {
				assert.Equal(t, "false", c.Annotations[capr.RKE2PrimeEnabledAnnotation])
				updated = append(updated, c.Name)
				return c, tc.updateErrs[c.Name]
			}).AnyTimes()

			cmClient := migConfigMap(false, rke2PrimeAnnotationMigratedKey)
			w := &wrangler.CAPIContext{Context: &wrangler.Context{
				Provisioning: migProvisioning(prov),
				Core:         &migCore{cm: cmClient},
			}}

			err := migrateExistingRKE2ClustersPrimeAnnotation(w)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantTokens, pager.Tokens())
			pager.AssertLimit(t)
			assert.Equal(t, tc.wantUpdated, updated)
			assert.Equal(t, tc.wantMigrated, isMigrated(cmClient, rke2PrimeAnnotationMigratedKey))
		})
	}
}

func TestMigrateMachinePoolsDynamicSchemaLabel_Processing(t *testing.T) {
	newCluster := func(name string, pools ...provv1.RKEMachinePool) provv1.Cluster {
		return provv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: migTestWorkspace},
			Spec:       provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{MachinePools: pools}},
		}
	}
	pool := func(kind string) provv1.RKEMachinePool {
		return provv1.RKEMachinePool{NodeConfig: &corev1.ObjectReference{Kind: kind}}
	}
	clusters := []provv1.Cluster{
		newCluster("c1", pool("Amazonec2Config")),
		{ObjectMeta: metav1.ObjectMeta{Name: "imported", Namespace: migTestWorkspace}}, // no RKE config
		newCluster("c3", pool("Amazonec2Config"), provv1.RKEMachinePool{DynamicSchemaSpec: "{}", NodeConfig: &corev1.ObjectReference{Kind: "Skip"}}),
		newCluster("nil-node-config", provv1.RKEMachinePool{}),
	}

	tests := []struct {
		name         string
		clusters     []provv1.Cluster
		schemaErr    error
		failPage     int
		wantErr      string
		wantTokens   []string
		wantUpdated  []string
		wantMigrated bool
	}{
		{
			name:         "machine pools on every page get the dynamic schema spec",
			clusters:     clusters[:3],
			wantTokens:   []string{"", "token-page-1"},
			wantUpdated:  []string{"c1", "c3"},
			wantMigrated: true,
		},
		{
			name:        "nil node config is an error",
			clusters:    clusters[2:],
			wantErr:     "node config must not be nil",
			wantTokens:  []string{""},
			wantUpdated: []string{"c3"},
		},
		{
			name:       "dynamic schema error stops the migration",
			clusters:   clusters[:1],
			schemaErr:  errMigBoom,
			wantErr:    "boom",
			wantTokens: []string{""},
		},
		{
			name:        "list error on page 2 does not mark the migration",
			clusters:    clusters[:3],
			failPage:    2,
			wantErr:     "simulated list error",
			wantTokens:  []string{"", "token-page-1"},
			wantUpdated: []string{"c1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := migSetup(t)
			mgmtPager := paginationtest.NewPager([]apiv3.Cluster{{Spec: apiv3.ClusterSpec{FleetWorkspaceName: migTestWorkspace}}}, 0)
			pager := paginationtest.NewPager(tc.clusters, 2).FailOnPage(tc.failPage, nil)
			prov := provClusterMock(ctrl, pager)

			var updated []string
			prov.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_, name string, _ metav1.GetOptions) (*provv1.Cluster, error) {
				for i := range tc.clusters {
					if tc.clusters[i].Name == name {
						return tc.clusters[i].DeepCopy(), nil
					}
				}
				return nil, errMigNotFound
			}).AnyTimes()
			prov.EXPECT().Update(gomock.Any()).DoAndReturn(func(c *provv1.Cluster) (*provv1.Cluster, error) {
				for _, p := range c.Spec.RKEConfig.MachinePools {
					assert.NotEmpty(t, p.DynamicSchemaSpec, "every machine pool needs a dynamic schema spec")
				}
				updated = append(updated, c.Name)
				return c, nil
			}).AnyTimes()

			schemas := fake.NewMockNonNamespacedControllerInterface[*apiv3.DynamicSchema, *apiv3.DynamicSchemaList](ctrl)
			schemas.EXPECT().Get("amazonec2config", gomock.Any()).DoAndReturn(func(string, metav1.GetOptions) (*apiv3.DynamicSchema, error) {
				if tc.schemaErr != nil {
					return nil, tc.schemaErr
				}
				return &apiv3.DynamicSchema{Spec: apiv3.DynamicSchemaSpec{SchemaName: "amazonec2config"}}, nil
			}).AnyTimes()

			cmClient := migConfigMap(false, dynamicSchemaMachinePoolsMigratedKey)
			w := &wrangler.CAPIContext{Context: &wrangler.Context{
				Provisioning: migProvisioning(prov),
				Mgmt:         &migMgmt{cluster: mgmtClusterMock(ctrl, mgmtPager), schema: schemas},
				Core:         &migCore{cm: cmClient},
			}}

			err := migrateMachinePoolsDynamicSchemaLabel(w)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantTokens, pager.Tokens())
			pager.AssertLimit(t)
			assert.Equal(t, tc.wantUpdated, updated)
			assert.Equal(t, tc.wantMigrated, isMigrated(cmClient, dynamicSchemaMachinePoolsMigratedKey))
		})
	}
}

// migCRTEnv records the calls of the CRT migration.
type migCRTEnv struct {
	created      []*corev1.Secret
	updated      []*apiv3.ClusterRegistrationToken
	createErr    error
	existing     *corev1.Secret
	getSecretErr error
	clusterErr   error
	crtGetErr    error
	crtUpdateErr error
}

func (e *migCRTEnv) wrangler(ctrl *gomock.Controller, crtList func(opts metav1.ListOptions) (*apiv3.ClusterRegistrationTokenList, error), cm *fakeMigrationConfigMapClient) *wrangler.Context {
	crts := fake.NewMockControllerInterface[*apiv3.ClusterRegistrationToken, *apiv3.ClusterRegistrationTokenList](ctrl)
	if crtList != nil {
		crts.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, opts metav1.ListOptions) (*apiv3.ClusterRegistrationTokenList, error) {
			return crtList(opts)
		}).AnyTimes()
	}
	crts.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ns, name string, _ metav1.GetOptions) (*apiv3.ClusterRegistrationToken, error) {
		if e.crtGetErr != nil {
			return nil, e.crtGetErr
		}
		return &apiv3.ClusterRegistrationToken{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
			Status:     apiv3.ClusterRegistrationTokenStatus{Token: "plain"},
		}, nil
	}).AnyTimes()
	crts.EXPECT().Update(gomock.Any()).DoAndReturn(func(c *apiv3.ClusterRegistrationToken) (*apiv3.ClusterRegistrationToken, error) {
		if e.crtUpdateErr != nil {
			return nil, e.crtUpdateErr
		}
		e.updated = append(e.updated, c)
		return c, nil
	}).AnyTimes()

	secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secrets.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
		e.created = append(e.created, s)
		if e.createErr != nil {
			return nil, e.createErr
		}
		return s, nil
	}).AnyTimes()
	secrets.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(string, string, metav1.GetOptions) (*corev1.Secret, error) {
		if e.getSecretErr != nil {
			return nil, e.getSecretErr
		}
		return e.existing, nil
	}).AnyTimes()

	// A NotFound error for the cluster skips the command generation.
	clusters := fake.NewMockNonNamespacedControllerInterface[*apiv3.Cluster, *apiv3.ClusterList](ctrl)
	clusters.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(string, metav1.GetOptions) (*apiv3.Cluster, error) {
		if e.clusterErr != nil {
			return nil, e.clusterErr
		}
		return nil, errMigNotFound
	}).AnyTimes()

	return &wrangler.Context{
		Mgmt: &migMgmt{cluster: clusters, crt: crts},
		Core: &migCore{cm: cm, secret: secrets},
	}
}

func newCRT(name, token, secretName string) apiv3.ClusterRegistrationToken {
	return apiv3.ClusterRegistrationToken{
		ObjectMeta: metav1.ObjectMeta{Namespace: "c-aaaaa", Name: name},
		Status:     apiv3.ClusterRegistrationTokenStatus{Token: token, TokenSecretName: secretName},
	}
}

func TestMigrateSingleCRT(t *testing.T) {
	tests := []struct {
		name        string
		crt         apiv3.ClusterRegistrationToken
		env         migCRTEnv
		wantErr     string
		wantCreated int
		wantUpdated int
	}{
		{
			name: "token secret already set does nothing",
			crt:  newCRT("crt", "plain", "existing-secret"),
		},
		{
			name: "empty token does nothing",
			crt:  newCRT("crt", "", ""),
		},
		{
			name:        "token moves to a secret and the CRT status is updated",
			crt:         newCRT("crt", "plain", ""),
			wantCreated: 1,
			wantUpdated: 1,
		},
		{
			name: "existing secret is reused",
			crt:  newCRT("crt", "plain", ""),
			env: migCRTEnv{
				createErr: apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, "crt-token-crt"),
				existing:  &corev1.Secret{Data: map[string][]byte{"expiresAt": []byte("2030-01-01T00:00:00Z")}},
			},
			wantCreated: 1,
			wantUpdated: 1,
		},
		{
			name:        "create error is wrapped",
			crt:         newCRT("crt", "plain", ""),
			env:         migCRTEnv{createErr: errMigBoom},
			wantErr:     "failed to create token secret",
			wantCreated: 1,
		},
		{
			name: "get existing secret error is wrapped",
			crt:  newCRT("crt", "plain", ""),
			env: migCRTEnv{
				createErr:    apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, "crt-token-crt"),
				getSecretErr: errMigBoom,
			},
			wantErr:     "failed to get existing secret",
			wantCreated: 1,
		},
		{
			name:        "cluster error is wrapped",
			crt:         newCRT("crt", "plain", ""),
			env:         migCRTEnv{clusterErr: errMigBoom},
			wantErr:     "failed to get cluster",
			wantCreated: 1,
		},
		{
			name:        "missing CRT is not an error",
			crt:         newCRT("crt", "plain", ""),
			env:         migCRTEnv{crtGetErr: errMigNotFound},
			wantCreated: 1,
		},
		{
			name:        "NotFound on CRT update is not an error",
			crt:         newCRT("crt", "plain", ""),
			env:         migCRTEnv{crtUpdateErr: errMigNotFound},
			wantCreated: 1,
		},
		{
			name:        "other CRT update error is returned",
			crt:         newCRT("crt", "plain", ""),
			env:         migCRTEnv{crtUpdateErr: errMigBoom},
			wantErr:     "boom",
			wantCreated: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := migSetup(t)
			env := tc.env
			w := env.wrangler(ctrl, nil, nil)

			err := migrateSingleCRT(w, &tc.crt)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Len(t, env.created, tc.wantCreated)
			require.Len(t, env.updated, tc.wantUpdated)
			for _, s := range env.created {
				assert.Equal(t, "crt-token-crt", s.Name)
				assert.Equal(t, []byte("plain"), s.Data["token"])
			}
			for _, u := range env.updated {
				assert.Empty(t, u.Status.Token, "the plaintext token must be removed")
				assert.Equal(t, "crt-token-crt", u.Status.TokenSecretName)
			}
		})
	}
}

func TestMigrateCRTTokensToSecretsFunc_Pagination(t *testing.T) {
	crts := []apiv3.ClusterRegistrationToken{
		newCRT("crt-1", "t1", ""),
		newCRT("crt-2", "t2", ""),
		newCRT("crt-3", "t3", ""),
	}

	tests := []struct {
		name         string
		failPage     int
		createErrOn  string // name of the secret that fails to create
		wantErr      string
		wantTokens   []string
		wantCreated  []string
		wantMigrated bool
	}{
		{
			name:         "CRTs on every page are migrated",
			wantTokens:   []string{"", "token-page-1"},
			wantCreated:  []string{"crt-token-crt-1", "crt-token-crt-2", "crt-token-crt-3"},
			wantMigrated: true,
		},
		{
			name:        "CRT error stops the migration",
			createErrOn: "crt-token-crt-2",
			wantErr:     "failed to create token secret",
			wantTokens:  []string{""},
			wantCreated: []string{"crt-token-crt-1", "crt-token-crt-2"},
		},
		{
			name:        "list error on page 2 does not mark the migration",
			failPage:    2,
			wantErr:     "simulated list error",
			wantTokens:  []string{"", "token-page-1"},
			wantCreated: []string{"crt-token-crt-1", "crt-token-crt-2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := migSetup(t)
			pager := paginationtest.NewPager(crts, 2).FailOnPage(tc.failPage, nil)
			cmClient := migConfigMap(false, crtTokensToSecretsMigratedKey)

			env := &migCRTEnv{}
			w := env.wrangler(ctrl, func(opts metav1.ListOptions) (*apiv3.ClusterRegistrationTokenList, error) {
				items, next, err := pager.Serve(opts)
				if err != nil {
					return nil, err
				}
				return &apiv3.ClusterRegistrationTokenList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}, nil
			}, cmClient)

			// Fail the create call for the secret named createErrOn.
			secrets := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			secrets.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
				env.created = append(env.created, s)
				if s.Name == tc.createErrOn {
					return nil, errMigBoom
				}
				return s, nil
			}).AnyTimes()
			w.Core.(*migCore).secret = secrets

			err := migrateCRTTokensToSecretsFunc(w)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantTokens, pager.Tokens())
			pager.AssertLimit(t)
			assert.Equal(t, tc.wantCreated, names(env.created, func(s *corev1.Secret) string { return s.Name }))
			assert.Equal(t, tc.wantMigrated, isMigrated(cmClient, crtTokensToSecretsMigratedKey))
		})
	}
}

// migInfraClient answers the CRD lookup of the infrastructure reference and reports NotFound for the object.
type migInfraClient struct {
	client.Client
}

func (migInfraClient) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if meta, ok := obj.(*metav1.PartialObjectMetadata); ok {
		meta.Labels = map[string]string{
			"cluster.x-k8s.io/v1beta2": "v1beta2",
			"cluster.x-k8s.io/v1beta1": "v1beta1",
		}
		return nil
	}
	return apierrors.NewNotFound(schema.GroupResource{Resource: "machines"}, "x")
}

func TestMigrateCAPIMachineLabelsAndAnnotationsToPlanSecret_Pagination(t *testing.T) {
	newMachine := func(name string) capi.Machine {
		m := capi.Machine{ObjectMeta: metav1.ObjectMeta{Namespace: migTestWorkspace, Name: name, Labels: map[string]string{migMachineLabel: name}}}
		m.Spec.Bootstrap.ConfigRef = capi.ContractVersionedObjectReference{APIGroup: capr.RKEAPIGroup, Kind: "RKEBootstrap", Name: "bootstrap-" + name}
		m.Spec.InfrastructureRef = capi.ContractVersionedObjectReference{APIGroup: "infrastructure.cluster.x-k8s.io", Kind: "DockerMachine", Name: name}
		return m
	}
	unmanaged := capi.Machine{ObjectMeta: metav1.ObjectMeta{Namespace: migTestWorkspace, Name: "no-bootstrap"}}

	provClusters := []provv1.Cluster{
		{ObjectMeta: metav1.ObjectMeta{Namespace: migTestWorkspace, Name: "p1"}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: migTestWorkspace, Name: "p2"}},
	}
	// Machines of each provisioning cluster by label selector.
	machines := map[string][]capi.Machine{
		labels.Set{capi.ClusterNameLabel: "p1"}.String(): {newMachine("m1a"), newMachine("m1b"), unmanaged},
		labels.Set{capr.ClusterNameLabel: "p1"}.String(): nil,
		labels.Set{capi.ClusterNameLabel: "p2"}.String(): nil,
		labels.Set{capr.ClusterNameLabel: "p2"}.String(): {newMachine("m2a")},
	}

	tests := []struct {
		name              string
		failMachineList   bool
		failSecretPage    int
		provListErr       error
		noPlanSecrets     bool
		wantErr           string
		wantMigrated      bool
		wantSecretsUpdate int
	}{
		{
			name:              "machines and plan secrets on every page are migrated",
			wantMigrated:      true,
			wantSecretsUpdate: 6,
		},
		{
			name:            "machine list error stops the migration",
			failMachineList: true,
			wantErr:         "simulated list error",
		},
		{
			name:          "machines without plan secrets are skipped completely",
			noPlanSecrets: true,
			wantMigrated:  true,
		},
		{
			name:           "plan secret list error stops the migration",
			failSecretPage: 2,
			wantErr:        "simulated list error",
		},
		{
			name:         "NotFound from the cluster list skips the workspace",
			provListErr:  errMigNotFound,
			wantMigrated: true,
		},
		{
			name:        "other cluster list error stops the migration",
			provListErr: errors.New("boom"),
			wantErr:     "boom",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := migSetup(t)
			mgmtPager := paginationtest.NewPager([]apiv3.Cluster{{Spec: apiv3.ClusterSpec{FleetWorkspaceName: migTestWorkspace}}}, 0)
			provPager := paginationtest.NewPager(provClusters, 1)
			if tc.provListErr != nil {
				provPager.FailOnPage(1, tc.provListErr)
			}
			prov := provClusterMock(ctrl, provPager)

			machinePagers := map[string]*paginationtest.Pager[capi.Machine]{}
			for selector, items := range machines {
				p := paginationtest.NewPager(items, 1)
				if tc.failMachineList {
					p.FailOnPage(1, nil)
				}
				machinePagers[selector] = p
			}
			machineCtrl := fake.NewMockControllerInterface[*capi.Machine, *capi.MachineList](ctrl)
			machineCtrl.EXPECT().List(migTestWorkspace, gomock.Any()).DoAndReturn(func(_ string, opts metav1.ListOptions) (*capi.MachineList, error) {
				p, ok := machinePagers[opts.LabelSelector]
				require.True(t, ok, "unexpected label selector %q", opts.LabelSelector)
				items, next, err := p.Serve(opts)
				if err != nil {
					return nil, err
				}
				return &capi.MachineList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}, nil
			}).AnyTimes()

			secretPagers := map[string]*paginationtest.Pager[corev1.Secret]{}
			var secretUpdates, bootstrapUpdates []string
			secretCtrl := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			secretCtrl.EXPECT().List(migTestWorkspace, gomock.Any()).DoAndReturn(func(_ string, opts metav1.ListOptions) (*corev1.SecretList, error) {
				p, ok := secretPagers[opts.LabelSelector]
				if !ok {
					var items []corev1.Secret
					names := []string{"1", "2"}
					if tc.noPlanSecrets {
						names = nil
					}
					for _, n := range names {
						items = append(items, corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: migTestWorkspace, Name: "plan-" + opts.LabelSelector + "-" + n}})
					}
					p = paginationtest.NewPager(items, 1).FailOnPage(tc.failSecretPage, nil)
					secretPagers[opts.LabelSelector] = p
				}
				items, next, err := p.Serve(opts)
				if err != nil {
					return nil, err
				}
				return &corev1.SecretList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}, nil
			}).AnyTimes()
			secretCtrl.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ns, name string, _ metav1.GetOptions) (*corev1.Secret, error) {
				return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, Labels: map[string]string{}, Annotations: map[string]string{}}}, nil
			}).AnyTimes()
			secretCtrl.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
				assert.NotEmpty(t, s.Labels[migMachineLabel], "the machine label must be copied")
				secretUpdates = append(secretUpdates, s.Name)
				return s, nil
			}).AnyTimes()

			bootstrapCtrl := fake.NewMockControllerInterface[*rkev1.RKEBootstrap, *rkev1.RKEBootstrapList](ctrl)
			bootstrapCtrl.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ns, name string, _ metav1.GetOptions) (*rkev1.RKEBootstrap, error) {
				return &rkev1.RKEBootstrap{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, Labels: map[string]string{}, Annotations: map[string]string{}}}, nil
			}).AnyTimes()
			bootstrapCtrl.EXPECT().Update(gomock.Any()).DoAndReturn(func(b *rkev1.RKEBootstrap) (*rkev1.RKEBootstrap, error) {
				assert.NotEmpty(t, b.Labels[migMachineLabel], "the machine label must be copied")
				bootstrapUpdates = append(bootstrapUpdates, b.Name)
				return b, nil
			}).AnyTimes()

			cmClient := migConfigMap(false, capiMigratedKey)
			w := &wrangler.CAPIContext{
				Context: &wrangler.Context{
					Provisioning: migProvisioning(prov),
					Mgmt:         &migMgmt{cluster: mgmtClusterMock(ctrl, mgmtPager)},
					Core:         &migCore{cm: cmClient, secret: secretCtrl},
					RKE:          &migRKE{bootstrap: bootstrapCtrl},
				},
				CAPI:   &migCAPI{machine: machineCtrl},
				Client: migInfraClient{},
			}

			err := migrateCAPIMachineLabelsAndAnnotationsToPlanSecret(w)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantMigrated, isMigrated(cmClient, capiMigratedKey))
			provPager.AssertLimit(t)
			if tc.provListErr != nil {
				for selector, p := range machinePagers {
					assert.Empty(t, p.Calls, "selector %q must not be listed", selector)
				}
				assert.Equal(t, []string{""}, provPager.Tokens())
				return
			}
			for selector, p := range machinePagers {
				p.AssertLimit(t)
				if tc.wantMigrated {
					assert.NotEmpty(t, p.Calls, "selector %q was never listed", selector)
				}
			}
			if tc.wantMigrated {
				// Provisioning clusters are listed in two pages.
				assert.Equal(t, []string{"", "token-page-1"}, provPager.Tokens())
				// The machine of p1 with the Cluster API label is listed in three pages.
				assert.Equal(t, []string{"", "token-page-1", "token-page-2"}, machinePagers[labels.Set{capi.ClusterNameLabel: "p1"}.String()].Tokens())
				if tc.noPlanSecrets {
					assert.Empty(t, secretUpdates)
					assert.Empty(t, bootstrapUpdates, "machines without plan secrets must not update the bootstrap")
					return
				}
				assert.Len(t, secretUpdates, tc.wantSecretsUpdate)
				assert.ElementsMatch(t, []string{"bootstrap-m1a", "bootstrap-m1b", "bootstrap-m2a"}, bootstrapUpdates)
				for _, p := range secretPagers {
					assert.Equal(t, []string{"", "token-page-1"}, p.Tokens(), "plan secrets must be read in two pages")
					p.AssertLimit(t)
				}
			}
		})
	}
}
