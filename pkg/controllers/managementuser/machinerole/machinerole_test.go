package machinerole

import (
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	provcluster "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestImportedLabelSyncSkipsAClusterCreatedAgainUnderTheSameName(t *testing.T) {
	ctrl := gomock.NewController(t)
	clusterCache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.Cluster](ctrl)
	clusterCache.EXPECT().Get("c-m-test").Return(&apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-new"},
		Status:     apimgmtv3.ClusterStatus{Driver: apimgmtv3.ClusterDriverRke2},
	}, nil)
	// No other clients: the nodes must not be labeled from the new cluster's machines.
	h := &handler{clusterName: "c-m-test", clusterUID: "uid-old", clusterCache: clusterCache}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{}, Annotations: map[string]string{}}}

	out, err := h.ImportedLabelSync("node-1", node)

	require.NoError(t, err)
	assert.Same(t, node, out)
}

// TestImportedLabelSyncV2provStampsCAPILabels is the companion half of the SURE-12151 /
// rancher/rancher#58320 regression guard (the planner half lives in config_test.go). It proves that
// for a v2prov cluster - which does NOT carry the CAPIClusterOwner labels - the controller resolves
// the backing CAPI Cluster/Machine via the provisioning-cluster index and stamps the CAPI-flavored
// plan.cattle.io/* labels onto the Node through the API. This is what lets the planner stop baking
// those labels into node-label on steady-state reconciles without losing them (snapshot restore and
// v2prov day-2 ops read them off the Node).
func TestImportedLabelSyncV2provStampsCAPILabels(t *testing.T) {
	const (
		mgmtName    = "c-m-test"
		capiName    = "my-cluster"
		capiNS      = "fleet-default"
		machineName = "my-cluster-pool-abc12"
		nodeName    = "node-1"
	)

	ctrl := gomock.NewController(t)

	clusterCache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.Cluster](ctrl)
	clusterCache.EXPECT().Get(mgmtName).Return(&apimgmtv3.Cluster{
		// No CAPIClusterOwner labels: this is v2prov, not turtles-imported.
		ObjectMeta: metav1.ObjectMeta{Name: mgmtName, UID: "uid-1"},
		Status:     apimgmtv3.ClusterStatus{Driver: apimgmtv3.ClusterDriverRke2},
	}, nil)

	// The mgmt cluster name is never the CAPI cluster name; it resolves through the provisioning cluster.
	provClusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
	provClusterCache.EXPECT().GetByIndex(provcluster.ByCluster, mgmtName).Return([]*provv1.Cluster{
		{ObjectMeta: metav1.ObjectMeta{Name: capiName, Namespace: capiNS}},
	}, nil)

	capiClusterCache := fake.NewMockCacheInterface[*capi.Cluster](ctrl)
	capiClusterCache.EXPECT().Get(capiNS, capiName).Return(&capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: capiName, Namespace: capiNS},
	}, nil)

	capiMachineCache := fake.NewMockCacheInterface[*capi.Machine](ctrl)
	capiMachineCache.EXPECT().List(capiNS, gomock.Any()).Return([]*capi.Machine{
		{
			ObjectMeta: metav1.ObjectMeta{Name: machineName, Namespace: capiNS,
				Labels: map[string]string{capi.ClusterNameLabel: capiName}},
			Status: capi.MachineStatus{NodeRef: capi.MachineNodeReference{Name: nodeName}},
		},
	}, nil)

	var updated *corev1.Node
	nodes := fake.NewMockNonNamespacedClientInterface[*corev1.Node, *corev1.NodeList](ctrl)
	nodes.EXPECT().Update(gomock.Any()).DoAndReturn(func(n *corev1.Node) (*corev1.Node, error) {
		updated = n
		return n, nil
	})

	h := &handler{
		clusterName:      mgmtName,
		clusterUID:       "uid-1",
		nodes:            nodes,
		clusterCache:     clusterCache,
		provClusterCache: provClusterCache,
		capiClusterCache: capiClusterCache,
		capiMachineCache: capiMachineCache,
	}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name: nodeName, Labels: map[string]string{}, Annotations: map[string]string{},
	}}

	_, err := h.ImportedLabelSync(nodeName, node)
	require.NoError(t, err)
	require.NotNil(t, updated, "expected the node to be updated with lifecycle labels")

	// Labels must be CAPI-flavored and match what the planner bakes into node-label at boot.
	assert.Equal(t, "cluster.x-k8s.io", updated.Labels["plan.cattle.io/cluster-group"])
	assert.Equal(t, "Cluster", updated.Labels["plan.cattle.io/cluster-kind"])
	assert.Equal(t, capiName, updated.Labels["plan.cattle.io/cluster-name"])
	assert.Equal(t, "cluster.x-k8s.io", updated.Labels["plan.cattle.io/machine-group"])
	assert.Equal(t, "Machine", updated.Labels["plan.cattle.io/machine-kind"])
	// The machine-* labels are what snapshot restore relies on.
	assert.Equal(t, machineName, updated.Labels["plan.cattle.io/machine-name"])
}
