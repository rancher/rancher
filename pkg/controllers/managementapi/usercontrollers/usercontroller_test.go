package usercontrollers

import (
	"context"
	"errors"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
)

func newMockUserControllersController(starter *simpleControllerStarter) *userControllersController {
	return &userControllersController{
		starter:       starter,
		clusterLister: &fakes.ClusterListerMock{},
		clusters: &fakes.ClusterInterfaceMock{
			ControllerFunc: func() v3.ClusterController {
				return &fakes.ClusterControllerMock{
					EnqueueFunc: func(namespace string, name string) {},
				}
			},
			UpdateFunc: func(c *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
				if c.Name == "bad cluster" {
					return c, errors.New("failed to update the cluster object")
				}
				return c, nil
			},
		},
	}
}

type simpleControllerStarter struct {
	startCalled bool
	stopCalled  bool
}

func (s *simpleControllerStarter) Start(ctx context.Context, c *v3.Cluster, clusterOwner bool) error {
	if c.Name == "nonstarter cluster" {
		return errors.New("failed to start the cluster controllers")
	}
	s.startCalled = true
	return nil
}

func (s *simpleControllerStarter) Stop(cluster *v3.Cluster) {
	s.stopCalled = true
}

func TestAnnotationFailsToBeSaved(t *testing.T) {
	t.Parallel()
	t.Run("initial annotation fails to be saved", func(t *testing.T) {
		t.Parallel()
		starter := simpleControllerStarter{}
		controller := newMockUserControllersController(&starter)
		cluster := &v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "bad cluster",
				Annotations: map[string]string{},
			},
			Status: apimgmtv3.ClusterStatus{
				Version: &version.Info{GitVersion: "1.23.4"},
			},
		}

		obj, err := controller.sync("", cluster)
		require.Error(t, err)
		require.Nil(t, obj)
		assert.False(t, starter.startCalled)
		assert.False(t, starter.stopCalled)
	})

	t.Run("new annotation fails to be saved after controllers restart", func(t *testing.T) {
		t.Parallel()
		starter := simpleControllerStarter{}
		controller := newMockUserControllersController(&starter)
		cluster := &v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "bad cluster",
				Annotations: map[string]string{currentClusterControllersVersion: "1.22.0"},
			},
			Status: apimgmtv3.ClusterStatus{
				Version: &version.Info{GitVersion: "1.23.4"},
			},
		}

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
	controller := newMockUserControllersController(&starter)
	cluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "nonstarter cluster",
			Annotations: map[string]string{currentClusterControllersVersion: "1.22.0"},
		},
		Status: apimgmtv3.ClusterStatus{
			Version: &version.Info{GitVersion: "1.23.4"},
		},
	}

	obj, err := controller.sync("", cluster)
	require.Error(t, err)
	require.NotNil(t, obj)
	assert.True(t, starter.stopCalled)
	assert.False(t, starter.startCalled)
	assert.Equal(t, "1.22.0", obj.(*v3.Cluster).Annotations[currentClusterControllersVersion])
}

func TestClusterWithoutControllersVersionAnnotationGetsUpdated(t *testing.T) {
	t.Parallel()
	starter := simpleControllerStarter{}
	controller := newMockUserControllersController(&starter)
	cluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-cluster",
			Annotations: map[string]string{},
		},
		Status: apimgmtv3.ClusterStatus{
			Version: &version.Info{GitVersion: "1.23.4"},
		},
	}

	obj, err := controller.sync("", cluster)
	require.NoError(t, err)
	require.NotNil(t, obj)
	assert.Equal(t, "1.23.4", obj.(*v3.Cluster).Annotations[currentClusterControllersVersion])
	assert.False(t, starter.startCalled)
	assert.False(t, starter.stopCalled)
}

func TestClusterControllersNotRestartedOnPatchVersionChange(t *testing.T) {
	t.Parallel()
	starter := simpleControllerStarter{}
	controller := newMockUserControllersController(&starter)
	cluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-cluster",
			Annotations: map[string]string{currentClusterControllersVersion: "1.23.0"},
		},
		Status: apimgmtv3.ClusterStatus{
			Version: &version.Info{GitVersion: "1.23.1"},
		},
	}

	obj, err := controller.sync("", cluster)
	require.NoError(t, err)
	require.NotNil(t, obj)

	// The annotation is also not updated.
	assert.Equal(t, "1.23.0", obj.(*v3.Cluster).Annotations[currentClusterControllersVersion])

	assert.False(t, starter.startCalled)
	assert.False(t, starter.stopCalled)
}

func TestClusterControllersWereStoppedAndStartedOnVersionChange(t *testing.T) {
	t.Parallel()
	t.Run("cluster version moved up", func(t *testing.T) {
		t.Parallel()
		starter := simpleControllerStarter{}
		controller := newMockUserControllersController(&starter)
		cluster := &v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "my-cluster",
				Annotations: map[string]string{currentClusterControllersVersion: "1.23.4"},
			},
			Status: apimgmtv3.ClusterStatus{
				Version: &version.Info{GitVersion: "1.25.2"},
			},
		}

		obj, err := controller.sync("", cluster)
		require.NoError(t, err)
		require.NotNil(t, obj)
		assert.Equal(t, "1.25.2", obj.(*v3.Cluster).Annotations[currentClusterControllersVersion])
		assert.True(t, starter.startCalled)
		assert.True(t, starter.stopCalled)
	})

	t.Run("cluster version moved down", func(t *testing.T) {
		t.Parallel()
		starter := simpleControllerStarter{}
		controller := newMockUserControllersController(&starter)
		cluster := &v3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "my-cluster",
				Annotations: map[string]string{currentClusterControllersVersion: "1.23.4"},
			},
			Status: apimgmtv3.ClusterStatus{
				Version: &version.Info{GitVersion: "1.22.1"},
			},
		}

		obj, err := controller.sync("", cluster)
		require.NoError(t, err)
		require.NotNil(t, obj)
		assert.Equal(t, "1.22.1", obj.(*v3.Cluster).Annotations[currentClusterControllersVersion])
		assert.True(t, starter.startCalled)
		assert.True(t, starter.stopCalled)
	})
}
