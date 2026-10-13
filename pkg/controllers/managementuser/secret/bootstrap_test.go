package secret

import (
	"errors"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func newBootstrapTestCluster(uid types.UID, resourceVersion string) *apimgmtv3.Cluster {
	return &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: uid, ResourceVersion: resourceVersion}}
}

// newBootstrapTestClusters returns a cluster client that rejects updates made from anything but the
// stored version of the cluster.
func newBootstrapTestClusters(stored *apimgmtv3.Cluster) *fakes.ClusterInterfaceMock {
	return &fakes.ClusterInterfaceMock{
		GetFunc: func(string, metav1.GetOptions) (*apimgmtv3.Cluster, error) {
			return stored.DeepCopy(), nil
		},
		UpdateStatusFunc: func(c *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
			if c.ResourceVersion != stored.ResourceVersion {
				return nil, apierrors.NewConflict(schema.GroupResource{Group: "management.cattle.io", Resource: "clusters"}, c.Name, errors.New("the object has been modified"))
			}
			stored = c.DeepCopy()
			return c, nil
		},
	}
}

func TestMarkPreBootstrappedUpdatesTheCluster(t *testing.T) {
	clusters := newBootstrapTestClusters(newBootstrapTestCluster("uid-1", "1"))
	cluster := newBootstrapTestCluster("uid-1", "1")

	require.NoError(t, markPreBootstrapped(clusters, cluster))

	require.Len(t, clusters.UpdateStatusCalls(), 1)
	assert.True(t, apimgmtv3.ClusterConditionPreBootstrapped.IsTrue(clusters.UpdateStatusCalls()[0].In1))
	assert.True(t, apimgmtv3.ClusterConditionPreBootstrapped.IsTrue(cluster), "the caller's copy should be marked too")
}

func TestMarkPreBootstrappedRetriesAStaleCopy(t *testing.T) {
	// The controllers are started with a copy of the cluster that is usually out of date by the time
	// pre-bootstrapping finishes.
	clusters := newBootstrapTestClusters(newBootstrapTestCluster("uid-1", "2"))

	require.NoError(t, markPreBootstrapped(clusters, newBootstrapTestCluster("uid-1", "1")))

	calls := clusters.UpdateStatusCalls()
	require.Len(t, calls, 2)
	assert.Equal(t, "2", calls[1].In1.ResourceVersion, "the retry should use the latest version")
	assert.True(t, apimgmtv3.ClusterConditionPreBootstrapped.IsTrue(calls[1].In1))
}

func TestMarkPreBootstrappedDoesNotTouchANewClusterWithTheSameName(t *testing.T) {
	clusters := newBootstrapTestClusters(newBootstrapTestCluster("uid-2", "2"))

	err := markPreBootstrapped(clusters, newBootstrapTestCluster("uid-1", "1"))

	require.Error(t, err)
	assert.Len(t, clusters.UpdateStatusCalls(), 1, "only the initial, conflicting update should be made")
}
