package dashboard

import (
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type localClusterMocks struct {
	clusters   *fake.MockNonNamespacedClientInterface[*v32.Cluster, *v32.ClusterList]
	namespaces *fake.MockNonNamespacedClientInterface[*corev1.Namespace, *corev1.NamespaceList]
}

func newLocalClusterMocks(t *testing.T) localClusterMocks {
	ctrl := gomock.NewController(t)
	return localClusterMocks{
		clusters:   fake.NewMockNonNamespacedClientInterface[*v32.Cluster, *v32.ClusterList](ctrl),
		namespaces: fake.NewMockNonNamespacedClientInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl),
	}
}

func setMCMAgent(t *testing.T, enabled bool) {
	previous := features.MCMAgent.Enabled()
	features.MCMAgent.Set(enabled)
	t.Cleanup(func() {
		features.MCMAgent.Set(previous)
	})
}

func createdCluster() *v32.Cluster {
	return &v32.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "local",
			UID:  "144ad316-4051-4152-adda-93602b1f4297",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: v32.SchemeGroupVersion.String(),
			Kind:       "Cluster",
		},
	}
}

func TestAddLocalClusterCreatesNamespace(t *testing.T) {
	setMCMAgent(t, false)
	mocks := newLocalClusterMocks(t)

	cluster := createdCluster()
	mocks.clusters.EXPECT().Create(gomock.Any()).Return(cluster, nil)
	mocks.clusters.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(c *v32.Cluster) (*v32.Cluster, error) {
		assert.Equal(t, v32.ClusterDriverImported, c.Status.Driver)
		return c, nil
	})

	var created *corev1.Namespace
	mocks.namespaces.EXPECT().Create(gomock.Any()).DoAndReturn(func(ns *corev1.Namespace) (*corev1.Namespace, error) {
		created = ns
		return ns, nil
	})

	require.NoError(t, addLocalCluster(false, mocks.clusters, mocks.namespaces))

	require.NotNil(t, created)
	assert.Equal(t, "local", created.Name)
	require.Len(t, created.OwnerReferences, 1)
	assert.Equal(t, metav1.OwnerReference{
		APIVersion: v32.SchemeGroupVersion.String(),
		Kind:       "Cluster",
		Name:       "local",
		UID:        cluster.UID,
	}, created.OwnerReferences[0])
}

func TestAddLocalClusterSkipsNamespaceForClusterAgent(t *testing.T) {
	setMCMAgent(t, true)
	mocks := newLocalClusterMocks(t)

	mocks.clusters.EXPECT().Create(gomock.Any()).Return(createdCluster(), nil)
	mocks.clusters.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(c *v32.Cluster) (*v32.Cluster, error) {
		return c, nil
	})
	// No namespace is expected to be created; gomock fails the test if one is.

	require.NoError(t, addLocalCluster(false, mocks.clusters, mocks.namespaces))
}

func TestAddLocalClusterEmbeddedDriver(t *testing.T) {
	setMCMAgent(t, false)
	mocks := newLocalClusterMocks(t)

	mocks.clusters.EXPECT().Create(gomock.Any()).Return(createdCluster(), nil)
	mocks.clusters.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(c *v32.Cluster) (*v32.Cluster, error) {
		assert.Equal(t, v32.ClusterDriverLocal, c.Status.Driver)
		return c, nil
	})
	mocks.namespaces.EXPECT().Create(gomock.Any()).Return(nil, nil)

	require.NoError(t, addLocalCluster(true, mocks.clusters, mocks.namespaces))
}

func TestAddLocalClusterToleratesExistingObjects(t *testing.T) {
	setMCMAgent(t, false)
	mocks := newLocalClusterMocks(t)

	existing := createdCluster()
	existing.Status.Conditions = []v32.ClusterCondition{{Type: "Ready", Status: corev1.ConditionTrue}}

	mocks.clusters.EXPECT().Create(gomock.Any()).Return(nil, alreadyExists("clusters", "local"))
	mocks.clusters.EXPECT().Get("local", gomock.Any()).Return(existing, nil)
	// The cluster is already Ready, so its status is left alone.
	mocks.namespaces.EXPECT().Create(gomock.Any()).Return(nil, alreadyExists("namespaces", "local"))

	require.NoError(t, addLocalCluster(false, mocks.clusters, mocks.namespaces))
}

func TestRemoveLocalCluster(t *testing.T) {
	mocks := newLocalClusterMocks(t)
	mocks.clusters.EXPECT().Delete("local", gomock.Any()).Return(nil)

	require.NoError(t, removeLocalCluster(mocks.clusters))
}

func alreadyExists(resource, name string) error {
	return apierrors.NewAlreadyExists(schema.GroupResource{Resource: resource}, name)
}
