package project_cluster

import (
	"testing"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// newRemoveTestLifecycle returns a cluster lifecycle for a cluster with no system project, whose namespace
// Get returns namespace, or NotFound if it's nil.
func newRemoveTestLifecycle(t *testing.T, namespace *corev1.Namespace) (*clusterLifecycle, *fake.MockNonNamespacedControllerInterface[*apisv3.Cluster, *apisv3.ClusterList]) {
	ctrl := gomock.NewController(t)

	projects := fake.NewMockControllerInterface[*apisv3.Project, *apisv3.ProjectList](ctrl)
	projects.EXPECT().WithImpersonation(gomock.Any()).Return(projects, nil).AnyTimes()
	projectLister := fake.NewMockCacheInterface[*apisv3.Project](ctrl)
	projectLister.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	nsClient := fake.NewMockNonNamespacedControllerInterface[*corev1.Namespace, *corev1.NamespaceList](ctrl)
	nsClient.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(func(name string, _ metav1.GetOptions) (*corev1.Namespace, error) {
		if namespace == nil {
			return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, name)
		}
		return namespace, nil
	}).AnyTimes()

	clusterClient := fake.NewMockNonNamespacedControllerInterface[*apisv3.Cluster, *apisv3.ClusterList](ctrl)
	return &clusterLifecycle{
		clusterClient: clusterClient,
		projects:      projects,
		projectLister: projectLister,
		nsClient:      nsClient,
	}, clusterClient
}

func newRemovingCluster() *apisv3.Cluster {
	now := metav1.Now()
	return &apisv3.Cluster{ObjectMeta: metav1.ObjectMeta{
		Name:              "c-m-test",
		DeletionTimestamp: &now,
		Finalizers:        []string{"controller.cattle.io/" + ClusterRemoveController},
	}}
}

func TestRemoveWaitsForTheNamespaceToBeGone(t *testing.T) {
	// A cluster created again under the same name must not find the old cluster's tokens, bindings and
	// projects in the namespace.
	lifecycle, clusterClient := newRemoveTestLifecycle(t, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating},
	})
	clusterClient.EXPECT().EnqueueAfter("c-m-test", namespaceRemovedRequeue)

	_, err := lifecycle.Remove(newRemovingCluster())

	assert.ErrorIs(t, err, generic.ErrSkip, "the finalizer should be kept")
}

func TestRemoveFinishesOnceTheNamespaceIsGone(t *testing.T) {
	lifecycle, _ := newRemoveTestLifecycle(t, nil)

	_, err := lifecycle.Remove(newRemovingCluster())

	require.NoError(t, err)
}
