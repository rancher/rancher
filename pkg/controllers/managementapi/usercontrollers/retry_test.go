package usercontrollers

import (
	"context"
	"errors"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/version"
)

func failedState(retryAt time.Time) clustermanager.ControllersState {
	return clustermanager.ControllersState{Phase: clustermanager.ControllersFailed, Err: errors.New("boom"), RetryAt: retryAt}
}

func TestSyncHoldsOffAFailedStartUntilItsBackoffIsOver(t *testing.T) {
	starter := simpleControllerStarter{state: failedState(time.Now().Add(time.Minute))}
	controller, clusters := newMockUserControllersController(t, &starter)
	cluster := newProvisionedCluster()

	clusters.EXPECT().EnqueueAfter(cluster.Name, gomock.Any()).Do(func(_ string, after time.Duration) {
		assert.Greater(t, after, time.Duration(0))
		assert.LessOrEqual(t, after, time.Minute)
	})

	obj, err := controller.sync(cluster.Name, cluster)

	require.NoError(t, err)
	assert.Same(t, cluster, obj)
	assert.False(t, starter.startCalled, "the start should wait for the backoff")
}

func TestSyncHoldsOffARestartUntilTheBackoffIsOver(t *testing.T) {
	// A version change restarts the controllers by stopping them first, which would forget the failure
	// and with it the backoff.
	starter := simpleControllerStarter{state: failedState(time.Now().Add(time.Minute))}
	controller, clusters := newMockUserControllersController(t, &starter)
	cluster := newProvisionedCluster()
	cluster.Annotations = map[string]string{currentClusterControllersVersion: "1.22.0"}
	cluster.Status.Version = &version.Info{GitVersion: "1.23.4"}

	clusters.EXPECT().EnqueueAfter(cluster.Name, gomock.Any())

	_, err := controller.sync(cluster.Name, cluster)

	require.NoError(t, err)
	assert.False(t, starter.stopCalled)
	assert.False(t, starter.startCalled)
}

func TestSyncRetriesAFailedStartOnceItsBackoffIsOver(t *testing.T) {
	starter := simpleControllerStarter{state: failedState(time.Now().Add(-time.Second))}
	controller, _ := newMockUserControllersController(t, &starter)

	_, err := controller.sync("c-m-test", newProvisionedCluster())

	require.NoError(t, err)
	assert.True(t, starter.startCalled)
}

func TestSyncStartsRightAwayAfterAFailureWithoutBackoff(t *testing.T) {
	// Failures that are returned from Start are backed off by the controller loop instead.
	starter := simpleControllerStarter{state: failedState(time.Time{})}
	controller, _ := newMockUserControllersController(t, &starter)

	_, err := controller.sync("c-m-test", newProvisionedCluster())

	require.NoError(t, err)
	assert.True(t, starter.startCalled)
}

func TestResyncClustersEnqueuesWithoutStarting(t *testing.T) {
	starter := simpleControllerStarter{}
	controller, clusters := newMockUserControllersController(t, &starter)
	lister := controller.clusterLister.(*fake.MockNonNamespacedCacheInterface[*v3.Cluster])

	provisioned := newProvisionedCluster()
	notProvisioned := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-new", UID: "uid-2"}}
	removing := newRemovingCluster("uid-3")
	removing.Name = "c-m-removing"
	v3.ClusterConditionProvisioned.True(removing)
	lister.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{provisioned, notProvisioned, removing}, nil)
	clusters.EXPECT().Enqueue(provisioned.Name)

	require.NoError(t, controller.resyncClusters())

	assert.False(t, starter.startCalled, "controllers are only started by sync")
}

func TestResyncClustersReturnsAFailureToList(t *testing.T) {
	controller, _ := newMockUserControllersController(t, &simpleControllerStarter{})
	lister := controller.clusterLister.(*fake.MockNonNamespacedCacheInterface[*v3.Cluster])
	lister.EXPECT().List(labels.Everything()).Return(nil, errors.New("boom"))

	assert.Error(t, controller.resyncClusters())
}

// resyncingOwnerStrategy owns every cluster and signals ownership changes through resync.
type resyncingOwnerStrategy struct {
	resync chan struct{}
}

func (s resyncingOwnerStrategy) isOwner(*v3.Cluster) bool      { return true }
func (s resyncingOwnerStrategy) forcedResync() <-chan struct{} { return s.resync }

func TestResyncOnOwnershipChangesResyncsEveryChange(t *testing.T) {
	controller, clusters := newMockUserControllersController(t, &simpleControllerStarter{})
	strategy := resyncingOwnerStrategy{resync: make(chan struct{})}
	controller.ownerStrategy = strategy
	lister := controller.clusterLister.(*fake.MockNonNamespacedCacheInterface[*v3.Cluster])
	cluster := newProvisionedCluster()
	lister.EXPECT().List(labels.Everything()).Return([]*v3.Cluster{cluster}, nil).Times(2)
	enqueued := make(chan string, 2)
	clusters.EXPECT().Enqueue(cluster.Name).Do(func(name string) { enqueued <- name }).Times(2)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		controller.resyncOnOwnershipChanges(ctx)
	}()

	for range 2 {
		strategy.resync <- struct{}{}
		select {
		case name := <-enqueued:
			assert.Equal(t, cluster.Name, name)
		case <-time.After(5 * time.Second):
			t.Fatal("the cluster was not resynced")
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("resyncing did not stop with its context")
	}
}

func TestResyncOnOwnershipChangesStopsWhenTheStrategyIsDone(t *testing.T) {
	controller, _ := newMockUserControllersController(t, &simpleControllerStarter{})
	strategy := resyncingOwnerStrategy{resync: make(chan struct{})}
	controller.ownerStrategy = strategy
	close(strategy.resync)

	done := make(chan struct{})
	go func() {
		defer close(done)
		controller.resyncOnOwnershipChanges(context.Background())
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("resyncing should stop once the strategy closes its channel, without resyncing")
	}
}
