package usercontrollers

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
)

func newMockUserControllersController(t *testing.T, starter *simpleControllerStarter) (*userControllersController, *fake.MockNonNamespacedClientInterface[*v3.Cluster, *v3.ClusterList]) {
	ctrl := gomock.NewController(t)
	clusterLister := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](ctrl)
	clusterClient := fake.NewMockNonNamespacedClientInterface[*v3.Cluster, *v3.ClusterList](ctrl)
	return &userControllersController{
		starter:       starter,
		clusterLister: clusterLister,
		clusters:      clusterClient,
		ownerStrategy: &nonClusteredStrategy{},
	}, clusterClient
}

type simpleControllerStarter struct {
	startCalled bool
	stopCalled  bool
}

func (s *simpleControllerStarter) Start(_ context.Context, c *v3.Cluster, _ bool) error {
	if c.Name == "nonstarter cluster" {
		return errors.New("failed to start the cluster controllers")
	}
	s.startCalled = true
	return nil
}

func (s *simpleControllerStarter) Stop(_ *v3.Cluster) {
	s.stopCalled = true
}

func TestAnnotationFailsToBeSaved(t *testing.T) {
	t.Parallel()
	t.Run("initial annotation fails to be saved", func(t *testing.T) {
		t.Parallel()
		starter := simpleControllerStarter{}
		controller, mockClient := newMockUserControllersController(t, &starter)
		cluster := &v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "bad cluster",
				Annotations: map[string]string{},
			},
			Status: v3.ClusterStatus{
				Version: &version.Info{GitVersion: "1.23.4"},
			},
		}
		v3.ClusterConditionProvisioned.True(cluster)

		mockClient.EXPECT().Update(gomock.Any()).Return(nil, errors.New("test error!"))
		obj, err := controller.sync("", cluster)
		require.Error(t, err)
		require.Nil(t, obj)
		assert.True(t, starter.startCalled)
		assert.False(t, starter.stopCalled)
	})

	t.Run("new annotation fails to be saved after controllers restart", func(t *testing.T) {
		t.Parallel()
		starter := simpleControllerStarter{}
		controller, mockClient := newMockUserControllersController(t, &starter)
		cluster := &v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "bad cluster",
				Annotations: map[string]string{currentClusterControllersVersion: "1.22.0"},
			},
			Status: v3.ClusterStatus{
				Version: &version.Info{GitVersion: "1.23.4"},
			},
		}
		v3.ClusterConditionProvisioned.True(cluster)

		mockClient.EXPECT().Update(gomock.Any()).Return(nil, errors.New("test error!"))
		obj, err := controller.sync("", cluster)
		require.Error(t, err)
		require.Nil(t, obj)
		assert.True(t, starter.startCalled)
		assert.True(t, starter.stopCalled)
	})
}

func TestClusterControllerFailsToRestart(t *testing.T) {
	t.Parallel()
	starter := simpleControllerStarter{}
	controller, _ := newMockUserControllersController(t, &starter)
	cluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "nonstarter cluster",
			Annotations: map[string]string{currentClusterControllersVersion: "1.22.0"},
		},
		Status: v3.ClusterStatus{
			Version: &version.Info{GitVersion: "1.23.4"},
		},
	}
	v3.ClusterConditionProvisioned.True(cluster)

	obj, err := controller.sync("", cluster)
	require.Error(t, err)
	require.Nil(t, obj)
	assert.True(t, starter.stopCalled)
	assert.False(t, starter.startCalled)
}

func TestClusterWithoutControllersVersionAnnotationGetsUpdated(t *testing.T) {
	t.Parallel()
	starter := simpleControllerStarter{}
	controller, mockClient := newMockUserControllersController(t, &starter)
	cluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-cluster",
			Annotations: map[string]string{},
		},
		Status: v3.ClusterStatus{
			Version: &version.Info{GitVersion: "1.23.4"},
		},
	}
	v3.ClusterConditionProvisioned.True(cluster)

	mockClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.Cluster) (*v3.Cluster, error) { return obj, nil })
	obj, err := controller.sync("", cluster)
	require.NoError(t, err)
	require.NotNil(t, obj)
	assert.Equal(t, "1.23.4", obj.Annotations[currentClusterControllersVersion])
	assert.True(t, starter.startCalled)
	assert.False(t, starter.stopCalled)
}

func TestClusterControllersNotRestartedOnPatchVersionChange(t *testing.T) {
	t.Parallel()
	starter := simpleControllerStarter{}
	controller, _ := newMockUserControllersController(t, &starter)
	cluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-cluster",
			Annotations: map[string]string{currentClusterControllersVersion: "1.23.0"},
		},
		Status: v3.ClusterStatus{
			Version: &version.Info{GitVersion: "1.23.1"},
		},
	}
	v3.ClusterConditionProvisioned.True(cluster)

	obj, err := controller.sync("", cluster)
	require.NoError(t, err)
	require.NotNil(t, obj)

	// The annotation is also not updated.
	assert.Equal(t, "1.23.0", obj.Annotations[currentClusterControllersVersion])

	assert.True(t, starter.startCalled)
	assert.False(t, starter.stopCalled)
}

func TestClusterControllersWereStoppedAndStartedOnVersionChange(t *testing.T) {
	t.Parallel()
	t.Run("cluster version moved up", func(t *testing.T) {
		t.Parallel()
		starter := simpleControllerStarter{}
		controller, mockClient := newMockUserControllersController(t, &starter)
		cluster := &v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "my-cluster",
				Annotations: map[string]string{currentClusterControllersVersion: "1.23.4"},
			},
			Status: v3.ClusterStatus{
				Version: &version.Info{GitVersion: "1.25.2"},
			},
		}
		v3.ClusterConditionProvisioned.True(cluster)

		mockClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.Cluster) (*v3.Cluster, error) { return obj, nil })
		obj, err := controller.sync("", cluster)
		require.NoError(t, err)
		require.NotNil(t, obj)
		assert.Equal(t, "1.25.2", obj.Annotations[currentClusterControllersVersion])
		assert.True(t, starter.startCalled)
		assert.True(t, starter.stopCalled)
	})

	t.Run("cluster version moved down", func(t *testing.T) {
		t.Parallel()
		starter := simpleControllerStarter{}
		controller, mockClient := newMockUserControllersController(t, &starter)
		cluster := &v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "my-cluster",
				Annotations: map[string]string{currentClusterControllersVersion: "1.23.4"},
			},
			Status: v3.ClusterStatus{
				Version: &version.Info{GitVersion: "1.22.1"},
			},
		}
		v3.ClusterConditionProvisioned.True(cluster)

		mockClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.Cluster) (*v3.Cluster, error) { return obj, nil })
		obj, err := controller.sync("", cluster)
		require.NoError(t, err)
		require.NotNil(t, obj)
		assert.Equal(t, "1.22.1", obj.Annotations[currentClusterControllersVersion])
		assert.True(t, starter.startCalled)
		assert.True(t, starter.stopCalled)
	})
}
