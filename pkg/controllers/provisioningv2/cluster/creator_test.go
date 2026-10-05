package cluster

import (
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type creatorTestEnv struct {
	clusters         *fake.MockControllerInterface[*v1.Cluster, *v1.ClusterList]
	clusterCache     *fake.MockCacheInterface[*v1.Cluster]
	mgmtClusters     *fake.MockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList]
	mgmtClusterCache *fake.MockNonNamespacedCacheInterface[*v3.Cluster]
}

// newCreatorTestHandler returns a handler whose clients fail the test on any call that isn't expected.
func newCreatorTestHandler(t *testing.T) (*handler, *creatorTestEnv) {
	ctrl := gomock.NewController(t)
	env := &creatorTestEnv{
		clusters:         fake.NewMockControllerInterface[*v1.Cluster, *v1.ClusterList](ctrl),
		clusterCache:     fake.NewMockCacheInterface[*v1.Cluster](ctrl),
		mgmtClusters:     fake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](ctrl),
		mgmtClusterCache: fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl),
	}
	return &handler{
		clusters:         env.clusters,
		clusterCache:     env.clusterCache,
		mgmtClusters:     env.mgmtClusters,
		mgmtClusterCache: env.mgmtClusterCache,
	}, env
}

func notFound(name string) error {
	return apierrors.NewNotFound(schema.GroupResource{Resource: "clusters"}, name)
}

func ownerAnnotations(gvk, namespace, name string) map[string]string {
	return map[string]string{apply.LabelGVK: gvk, apply.LabelNamespace: namespace, apply.LabelName: name}
}

func deleting(meta *metav1.ObjectMeta) {
	now := metav1.NewTime(time.Now())
	meta.DeletionTimestamp = &now
}

// newV2ProvCluster returns a provisioning cluster that creates the management cluster c-m-test.
func newV2ProvCluster(finalizers ...string) *v1.Cluster {
	return &v1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "foo", UID: "prov-uid", Finalizers: finalizers},
		Status:     v1.ClusterStatus{ClusterName: "c-m-test"},
	}
}

// newCreatedMgmtCluster returns the management cluster c-m-test, created by the provisioning cluster
// namespace/name.
func newCreatedMgmtCluster(uid types.UID, namespace, name string) *v3.Cluster {
	return &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: uid, Annotations: ownerAnnotations(util.ProvisioningClusterGVK, namespace, name)},
		Spec:       v3.ClusterSpec{FleetWorkspaceName: namespace},
	}
}

func TestGenerateLegacyClusterAddsItsFinalizerBeforeCreatingTheManagementCluster(t *testing.T) {
	h, env := newCreatorTestHandler(t)
	cluster := newV2ProvCluster()
	cluster.Name = "my-cluster"
	var updated *v1.Cluster
	env.clusters.EXPECT().Update(gomock.Any()).DoAndReturn(func(c *v1.Cluster) (*v1.Cluster, error) {
		updated = c
		return c, nil
	})

	objs, _, err := h.generateLegacyClusterFromProvisioningCluster(cluster, cluster.Status)

	assert.ErrorIs(t, err, errFinalizerAdded)
	assert.Empty(t, objs, "nothing should be created before the finalizer is in place")
	require.NotNil(t, updated)
	assert.Contains(t, updated.Finalizers, removeManagementClusterFinalizer)
	assert.Empty(t, cluster.Finalizers, "the cached object must not be modified")
}

func TestGenerateProvisioningClusterAddsItsFinalizerBeforeCreatingTheProvisioningCluster(t *testing.T) {
	h, env := newCreatorTestHandler(t)
	cluster := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-12345"}, Spec: v3.ClusterSpec{FleetWorkspaceName: "fleet-default"}}
	var updated *v3.Cluster
	env.mgmtClusters.EXPECT().Update(gomock.Any()).DoAndReturn(func(c *v3.Cluster) (*v3.Cluster, error) {
		updated = c
		return c, nil
	})

	objs, _, err := h.generateProvisioningClusterFromLegacyCluster(cluster, cluster.Status)

	assert.ErrorIs(t, err, errFinalizerAdded)
	assert.Empty(t, objs, "nothing should be created before the finalizer is in place")
	require.NotNil(t, updated)
	assert.Contains(t, updated.Finalizers, removeProvisioningClusterFinalizer)
}

func TestCheckManagementClusterAdoptable(t *testing.T) {
	cluster := newV2ProvCluster()
	tests := []struct {
		name    string
		mgmt    *v3.Cluster
		wantErr string
	}{
		{name: "no management cluster yet"},
		{name: "created by this provisioning cluster", mgmt: newCreatedMgmtCluster("uid-1", "fleet-default", "foo")},
		{name: "created by nothing", mgmt: &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}}},
		{
			name:    "created by another provisioning cluster",
			mgmt:    newCreatedMgmtCluster("uid-1", "fleet-default", "bar"),
			wantErr: "management cluster c-m-test belongs to provisioning cluster fleet-default/bar",
		},
		{
			name: "being removed",
			mgmt: func() *v3.Cluster {
				c := newCreatedMgmtCluster("uid-1", "fleet-default", "foo")
				deleting(&c.ObjectMeta)
				return c
			}(),
			wantErr: "waiting for management cluster c-m-test to be removed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, env := newCreatorTestHandler(t)
			if tt.mgmt == nil {
				env.mgmtClusterCache.EXPECT().Get("c-m-test").Return(nil, notFound("c-m-test"))
			} else {
				env.mgmtClusterCache.EXPECT().Get("c-m-test").Return(tt.mgmt, nil)
			}
			env.clusters.EXPECT().EnqueueAfter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			err := h.checkManagementClusterAdoptable(cluster, "c-m-test")

			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestOnProvisioningClusterRemoveWaitsForTheMachines(t *testing.T) {
	h, env := newCreatorTestHandler(t)
	cluster := newV2ProvCluster(removeManagementClusterFinalizer, util.ProvisioningClusterRemoveFinalizer)
	deleting(&cluster.ObjectMeta)
	env.clusters.EXPECT().EnqueueAfter("fleet-default", "foo", gomock.Any())

	obj, err := h.onProvisioningClusterRemove("", cluster)

	require.NoError(t, err)
	assert.Same(t, cluster, obj, "the management cluster must stay while machines drain through it")
}

func TestOnProvisioningClusterRemoveDeletesTheManagementClusterItCreated(t *testing.T) {
	h, env := newCreatorTestHandler(t)
	cluster := newV2ProvCluster(removeManagementClusterFinalizer)
	deleting(&cluster.ObjectMeta)
	env.mgmtClusterCache.EXPECT().Get("c-m-test").Return(newCreatedMgmtCluster("uid-1", "fleet-default", "foo"), nil)
	env.mgmtClusters.EXPECT().Delete("c-m-test", gomock.Any()).DoAndReturn(func(_ string, opts *metav1.DeleteOptions) error {
		require.NotNil(t, opts.Preconditions)
		assert.Equal(t, types.UID("uid-1"), *opts.Preconditions.UID, "only the management cluster that was looked at may be deleted")
		return nil
	})
	env.clusters.EXPECT().EnqueueAfter("fleet-default", "foo", gomock.Any())

	obj, err := h.onProvisioningClusterRemove("", cluster)

	require.NoError(t, err)
	assert.Same(t, cluster, obj, "the finalizer stays until the management cluster is gone")
}

func TestOnProvisioningClusterRemoveFinishesOnceTheManagementClusterIsGone(t *testing.T) {
	for name, mgmt := range map[string]*v3.Cluster{
		"gone": nil,
		// Moved to another fleet workspace, or taken over by another provisioning cluster: not ours to delete.
		"created by another provisioning cluster": newCreatedMgmtCluster("uid-1", "fleet-default", "bar"),
	} {
		t.Run(name, func(t *testing.T) {
			h, env := newCreatorTestHandler(t)
			cluster := newV2ProvCluster(removeManagementClusterFinalizer, "other-finalizer")
			deleting(&cluster.ObjectMeta)
			if mgmt == nil {
				env.mgmtClusterCache.EXPECT().Get("c-m-test").Return(nil, notFound("c-m-test"))
			} else {
				env.mgmtClusterCache.EXPECT().Get("c-m-test").Return(mgmt, nil)
			}
			var updated *v1.Cluster
			env.clusters.EXPECT().Update(gomock.Any()).DoAndReturn(func(c *v1.Cluster) (*v1.Cluster, error) {
				updated = c
				return c, nil
			})

			_, err := h.onProvisioningClusterRemove("", cluster)

			require.NoError(t, err)
			require.NotNil(t, updated)
			assert.Equal(t, []string{"other-finalizer"}, updated.Finalizers)
		})
	}
}

func TestOnProvisioningClusterRemoveIgnoresClustersNotBeingRemoved(t *testing.T) {
	h, _ := newCreatorTestHandler(t)
	cluster := newV2ProvCluster(removeManagementClusterFinalizer)

	obj, err := h.onProvisioningClusterRemove("", cluster)

	require.NoError(t, err)
	assert.Same(t, cluster, obj)
}

// newLegacyMgmtCluster returns a management cluster that creates the provisioning cluster
// fleet-default/c-12345.
func newLegacyMgmtCluster(finalizers ...string) *v3.Cluster {
	cluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-12345", UID: "mgmt-uid", Finalizers: finalizers},
		Spec:       v3.ClusterSpec{FleetWorkspaceName: "fleet-default"},
	}
	deleting(&cluster.ObjectMeta)
	return cluster
}

func TestOnManagementClusterRemoveDeletesTheProvisioningClusterItCreated(t *testing.T) {
	h, env := newCreatorTestHandler(t)
	cluster := newLegacyMgmtCluster(removeProvisioningClusterFinalizer)
	env.clusterCache.EXPECT().Get("fleet-default", "c-12345").Return(&v1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "c-12345", UID: "prov-uid", Annotations: ownerAnnotations(util.ManagementClusterGVK, "", "c-12345")},
	}, nil)
	env.clusters.EXPECT().Delete("fleet-default", "c-12345", gomock.Any()).DoAndReturn(func(_, _ string, opts *metav1.DeleteOptions) error {
		require.NotNil(t, opts.Preconditions)
		assert.Equal(t, types.UID("prov-uid"), *opts.Preconditions.UID)
		return nil
	})
	env.mgmtClusters.EXPECT().EnqueueAfter("c-12345", gomock.Any())

	obj, err := h.onManagementClusterRemove("", cluster)

	require.NoError(t, err)
	assert.Same(t, cluster, obj, "the finalizer stays until the provisioning cluster is gone")
}

func TestOnManagementClusterRemoveFinishesOnceTheProvisioningClusterIsGone(t *testing.T) {
	for name, prov := range map[string]*v1.Cluster{
		"gone":                       nil,
		"not created by the cluster": {ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "c-12345"}},
	} {
		t.Run(name, func(t *testing.T) {
			h, env := newCreatorTestHandler(t)
			cluster := newLegacyMgmtCluster(removeProvisioningClusterFinalizer, "other-finalizer")
			if prov == nil {
				env.clusterCache.EXPECT().Get("fleet-default", "c-12345").Return(nil, notFound("c-12345"))
			} else {
				env.clusterCache.EXPECT().Get("fleet-default", "c-12345").Return(prov, nil)
			}
			var updated *v3.Cluster
			env.mgmtClusters.EXPECT().Update(gomock.Any()).DoAndReturn(func(c *v3.Cluster) (*v3.Cluster, error) {
				updated = c
				return c, nil
			})

			_, err := h.onManagementClusterRemove("", cluster)

			require.NoError(t, err)
			require.NotNil(t, updated)
			assert.Equal(t, []string{"other-finalizer"}, updated.Finalizers)
		})
	}
}

func TestOnMgmtClusterRemoveAsksForItsCreatorToBeDeletedWithoutWaiting(t *testing.T) {
	h, env := newCreatorTestHandler(t)
	mgmtCluster := newCreatedMgmtCluster("uid-1", "fleet-default", "foo")
	deleting(&mgmtCluster.ObjectMeta)
	env.clusterCache.EXPECT().Get("fleet-default", "foo").Return(newV2ProvCluster(), nil)
	env.clusters.EXPECT().Delete("fleet-default", "foo", gomock.Any()).Return(nil)

	obj, err := h.OnMgmtClusterRemove("", mgmtCluster)

	require.NoError(t, err, "the management cluster must not wait for its creator")
	assert.Same(t, mgmtCluster, obj)
}

func TestOnMgmtClusterRemoveLeavesOtherProvisioningClustersAlone(t *testing.T) {
	// A provisioning cluster that only names the management cluster, but didn't create it, is a
	// replacement waiting for it to be removed.
	h, _ := newCreatorTestHandler(t)
	mgmtCluster := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}}
	deleting(&mgmtCluster.ObjectMeta)

	obj, err := h.OnMgmtClusterRemove("", mgmtCluster)

	require.NoError(t, err)
	assert.Same(t, mgmtCluster, obj)
}
