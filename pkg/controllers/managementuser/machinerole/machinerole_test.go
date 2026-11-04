package machinerole

import (
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
