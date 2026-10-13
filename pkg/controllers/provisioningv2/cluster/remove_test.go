package cluster

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// withUnsyncedCAPI makes doClusterRemove stop right after deciding what to do with the management
// cluster: the CAPI cluster cache isn't synced, so it asks to be retried.
func withUnsyncedCAPI(t *testing.T, h *handler, env *creatorTestEnv, cluster *v1.Cluster) {
	ctrl := gomock.NewController(t)
	capiClusters := fake.NewMockControllerInterface[*capi.Cluster, *capi.ClusterList](ctrl)
	capiClusters.EXPECT().Informer().Return(cache.NewSharedIndexInformer(&cache.ListWatch{}, &capi.Cluster{}, 0, cache.Indexers{}))
	h.capiClusters = capiClusters
	env.clusters.EXPECT().EnqueueAfter(cluster.Namespace, cluster.Name, gomock.Any())
}

func TestDoClusterRemoveAsksForTheCreatingManagementClusterToBeDeleted(t *testing.T) {
	// A provisioning cluster created by its management cluster would be created again: the management
	// cluster is deleted too, without waiting for it.
	h, env := newCreatorTestHandler(t)
	cluster := &v1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "c-12345", Annotations: ownerAnnotations(util.ManagementClusterGVK, "", "c-12345")},
		Status:     v1.ClusterStatus{ClusterName: "c-12345"},
	}
	env.mgmtClusters.EXPECT().Get("c-12345", gomock.Any()).Return(&v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-12345", UID: "mgmt-uid"}}, nil)
	env.mgmtClusters.EXPECT().Delete("c-12345", gomock.Any()).DoAndReturn(func(_ string, opts *metav1.DeleteOptions) error {
		require.NotNil(t, opts.Preconditions)
		assert.Equal(t, types.UID("mgmt-uid"), *opts.Preconditions.UID)
		return nil
	})
	withUnsyncedCAPI(t, h, env, cluster)

	_, err := h.doClusterRemove(cluster)()

	assert.ErrorIs(t, err, generic.ErrSkip)
}

func TestDoClusterRemoveLeavesTheManagementClusterItCreated(t *testing.T) {
	// The management cluster a provisioning cluster created is removed by removeManagementClusterFinalizer,
	// once the machines are gone: they are drained through its tunnel.
	h, env := newCreatorTestHandler(t)
	cluster := newV2ProvCluster(removeManagementClusterFinalizer)
	env.mgmtClusters.EXPECT().Get("c-m-test", gomock.Any()).Return(newCreatedMgmtCluster("uid-1", "fleet-default", "foo"), nil)
	featureCache := fake.NewMockNonNamespacedCacheInterface[*v3.Feature](gomock.NewController(t))
	featureCache.EXPECT().Get(features.RKE2.Name()).Return(&v3.Feature{}, nil)
	h.featureCache = featureCache
	withUnsyncedCAPI(t, h, env, cluster)

	_, err := h.doClusterRemove(cluster)()

	assert.ErrorIs(t, err, generic.ErrSkip)
	// The mock fails the test if the management cluster is deleted.
}
