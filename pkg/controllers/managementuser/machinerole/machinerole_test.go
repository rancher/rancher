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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// TestImportedLabelSyncV2provStampsCAPILabels is the controller half of the rancher/rancher#58320
// guard (the planner half is in config_test.go). For a v2prov cluster, which has no CAPIClusterOwner
// labels, the controller resolves the backing CAPI Cluster/Machine via the provisioning-cluster
// index and stamps the CAPI plan.cattle.io/* labels onto the Node over the API. That's what lets the
// planner stop putting those labels in node-label without losing them.
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

	// The mgmt cluster name isn't the CAPI cluster name; it resolves through the provisioning cluster.
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

	// Labels must be the CAPI ones, matching what the planner puts in node-label at boot.
	assert.Equal(t, "cluster.x-k8s.io", updated.Labels["plan.cattle.io/cluster-group"])
	assert.Equal(t, "Cluster", updated.Labels["plan.cattle.io/cluster-kind"])
	assert.Equal(t, capiName, updated.Labels["plan.cattle.io/cluster-name"])
	assert.Equal(t, "cluster.x-k8s.io", updated.Labels["plan.cattle.io/machine-group"])
	assert.Equal(t, "Machine", updated.Labels["plan.cattle.io/machine-kind"])
	// The machine-* labels are what snapshot restore uses.
	assert.Equal(t, machineName, updated.Labels["plan.cattle.io/machine-name"])
}

// TestImportedLabelSyncImportedFallsBackToMgmtV3 guards the imported-cluster path. A plain imported
// RKE2/K3s cluster has no CAPIClusterOwner labels but still has a provisioning.cattle.io Cluster
// shell, so capiClusterRef resolves a reference even though there's no cluster.x-k8s.io CAPI Cluster.
// The handler must fall through to the mgmt v3 Cluster + mgmt v3 Node and still stamp the labels.
// Returning early on the NotFound CAPI cluster (the earlier bug) left imported Nodes unlabelled,
// which broke etcd snapshot back-population.
func TestImportedLabelSyncImportedFallsBackToMgmtV3(t *testing.T) {
	const (
		mgmtName     = "c-m-test"
		provName     = "imported-cluster"
		provNS       = "fleet-default"
		mgmtNodeName = "machine-xyz"
		nodeName     = "node-1"
	)

	ctrl := gomock.NewController(t)

	clusterCache := fake.NewMockNonNamespacedCacheInterface[*apimgmtv3.Cluster](ctrl)
	clusterCache.EXPECT().Get(mgmtName).Return(&apimgmtv3.Cluster{
		TypeMeta:   metav1.TypeMeta{Kind: "Cluster", APIVersion: apimgmtv3.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{Name: mgmtName, UID: "uid-1"},
		Status:     apimgmtv3.ClusterStatus{Driver: apimgmtv3.ClusterDriverRke2},
	}, nil)

	// Imported clusters still have a provisioning cluster shell, so the ref resolves,
	provClusterCache := fake.NewMockCacheInterface[*provv1.Cluster](ctrl)
	provClusterCache.EXPECT().GetByIndex(provcluster.ByCluster, mgmtName).Return([]*provv1.Cluster{
		{ObjectMeta: metav1.ObjectMeta{Name: provName, Namespace: provNS}},
	}, nil)

	// but there is no CAPI Cluster: the Get must not abort the sync.
	capiClusterCache := fake.NewMockCacheInterface[*capi.Cluster](ctrl)
	capiClusterCache.EXPECT().Get(provNS, provName).Return(nil,
		apierrors.NewNotFound(schema.GroupResource{Group: capi.GroupVersion.Group, Resource: "clusters"}, provName))

	// Fallback resolves the mgmt v3 Node for this downstream node.
	mgmtNodeCache := fake.NewMockCacheInterface[*apimgmtv3.Node](ctrl)
	mgmtNodeCache.EXPECT().List(mgmtName, gomock.Any()).Return([]*apimgmtv3.Node{
		{
			TypeMeta:   metav1.TypeMeta{Kind: "Node", APIVersion: apimgmtv3.SchemeGroupVersion.String()},
			ObjectMeta: metav1.ObjectMeta{Name: mgmtNodeName, Namespace: mgmtName},
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
		mgmtNodeCache:    mgmtNodeCache,
		provClusterCache: provClusterCache,
		capiClusterCache: capiClusterCache,
	}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name: nodeName, Labels: map[string]string{}, Annotations: map[string]string{},
	}}

	_, err := h.ImportedLabelSync(nodeName, node)
	require.NoError(t, err)
	require.NotNil(t, updated, "imported node must still be labelled via the mgmt v3 fallback, not skipped")

	// mgmt v3 identity (what snapshotbackpopulate expects for imported clusters).
	assert.Equal(t, "management.cattle.io", updated.Labels["plan.cattle.io/machine-group"])
	assert.Equal(t, "Node", updated.Labels["plan.cattle.io/machine-kind"])
	assert.Equal(t, mgmtNodeName, updated.Labels["plan.cattle.io/machine-name"])
	assert.Equal(t, mgmtName, updated.Labels["plan.cattle.io/cluster-name"])
}
