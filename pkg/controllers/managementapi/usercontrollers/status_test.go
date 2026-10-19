package usercontrollers

import (
	"context"
	"errors"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newStatusTestController(owner bool, state clustermanager.ControllersState) *userControllersController {
	return &userControllersController{
		starter:       &simpleControllerStarter{state: state},
		ownerStrategy: fixedOwnerStrategy{owner: owner},
	}
}

func newProvisionedCluster() *v3.Cluster {
	c := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-1"}}
	v3.ClusterConditionProvisioned.True(c)
	return c
}

// syncTestStatus runs syncStatus the way the status handler does, on a copy of cluster.
func syncTestStatus(t *testing.T, u *userControllersController, cluster *v3.Cluster) *v3.Cluster {
	t.Helper()
	obj := cluster.DeepCopy()
	status, err := u.syncStatus(obj, obj.Status)
	require.NoError(t, err)
	obj.Status = status
	return obj
}

func TestSyncStatusMarksRunningControllers(t *testing.T) {
	u := newStatusTestController(true, clustermanager.ControllersState{Phase: clustermanager.ControllersRunning, Owner: true, PreBootstrapped: true})

	got := syncTestStatus(t, u, newProvisionedCluster())

	assert.True(t, v3.ClusterConditionControllersRunning.IsTrue(got))
	assert.True(t, v3.ClusterConditionPreBootstrapped.IsTrue(got))
	assert.NotEmpty(t, v3.ClusterConditionReady.GetStatus(got), "Ready should be summarized after a change")
}

func TestSyncStatusMarksFailedControllers(t *testing.T) {
	u := newStatusTestController(true, clustermanager.ControllersState{Phase: clustermanager.ControllersFailed, Err: errors.New("boom")})
	cluster := newProvisionedCluster()
	v3.ClusterConditionControllersRunning.True(cluster)

	got := syncTestStatus(t, u, cluster)

	assert.True(t, v3.ClusterConditionControllersRunning.IsFalse(got))
	assert.False(t, v3.ClusterConditionReady.IsTrue(got))
}

func TestSyncStatusLeavesTheConditionsAloneWhileStarting(t *testing.T) {
	// What the last start reported holds until this one is over.
	u := newStatusTestController(true, clustermanager.ControllersState{Phase: clustermanager.ControllersStarting, Owner: true})
	cluster := newProvisionedCluster()
	v3.ClusterConditionControllersRunning.False(cluster)

	got := syncTestStatus(t, u, cluster)

	assert.True(t, v3.ClusterConditionControllersRunning.IsFalse(got))
	assert.True(t, v3.ClusterConditionPreBootstrapped.IsUnknown(got), "a missing PreBootstrapped condition is created as Unknown")
}

func TestSyncStatusDoesNotMarkControllersRunningAsAFollower(t *testing.T) {
	// This replica just became the owner and its controllers haven't been restarted as the owner yet.
	u := newStatusTestController(true, clustermanager.ControllersState{Phase: clustermanager.ControllersRunning, Owner: false})

	got := syncTestStatus(t, u, newProvisionedCluster())

	assert.Empty(t, v3.ClusterConditionControllersRunning.GetStatus(got))
}

func TestSyncStatusNeverRegressesPreBootstrapped(t *testing.T) {
	// A new start hasn't pre-bootstrapped the cluster yet, but an earlier one did.
	u := newStatusTestController(true, clustermanager.ControllersState{Phase: clustermanager.ControllersStarting, Owner: true})
	cluster := newProvisionedCluster()
	v3.ClusterConditionPreBootstrapped.True(cluster)

	got := syncTestStatus(t, u, cluster)

	assert.True(t, v3.ClusterConditionPreBootstrapped.IsTrue(got))
}

func TestSyncStatusDoesNotReport(t *testing.T) {
	running := clustermanager.ControllersState{Phase: clustermanager.ControllersRunning, Owner: true, PreBootstrapped: true}
	now := metav1.NewTime(time.Now())
	tests := []struct {
		name    string
		owner   bool
		state   clustermanager.ControllersState
		cluster func() *v3.Cluster
	}{
		{
			name:    "when not the owner",
			owner:   false,
			state:   running,
			cluster: newProvisionedCluster,
		},
		{
			name:  "for a cluster that is not provisioned",
			owner: true,
			state: running,
			cluster: func() *v3.Cluster {
				return &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-1"}}
			},
		},
		{
			name:  "for a cluster being removed",
			owner: true,
			state: running,
			cluster: func() *v3.Cluster {
				c := newProvisionedCluster()
				c.DeletionTimestamp = &now
				return c
			},
		},
		{
			name:    "when no controllers run on this replica",
			owner:   true,
			state:   clustermanager.ControllersState{Phase: clustermanager.ControllersStopped},
			cluster: newProvisionedCluster,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newStatusTestController(tt.owner, tt.state)
			cluster := tt.cluster()

			got := syncTestStatus(t, u, cluster)

			assert.Equal(t, cluster.Status, got.Status)
		})
	}
}

// registerTestStatusHandler registers syncStatus the way Register does and returns the handler the
// cluster controller would run, along with the controller to set expectations on.
func registerTestStatusHandler(t *testing.T, u *userControllersController) (generic.Handler, *fake.MockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList]) {
	t.Helper()
	ctrl := gomock.NewController(t)
	clusters := fake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](ctrl)
	var handler generic.Handler
	clusters.EXPECT().AddGenericHandler(gomock.Any(), "user-controllers-status", gomock.Any()).Do(func(_ context.Context, _ string, h generic.Handler) {
		handler = h
	})
	controllers.RegisterClusterStatusHandler(context.Background(), clusters, "", "user-controllers-status", u.syncStatus)
	require.NotNil(t, handler)
	return handler, clusters
}

func TestStatusHandlerUpdatesTheStatusOnChange(t *testing.T) {
	u := newStatusTestController(true, clustermanager.ControllersState{Phase: clustermanager.ControllersRunning, Owner: true, PreBootstrapped: true})
	handler, clusters := registerTestStatusHandler(t, u)
	cluster := newProvisionedCluster()

	clusters.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(c *v3.Cluster) (*v3.Cluster, error) {
		assert.True(t, v3.ClusterConditionControllersRunning.IsTrue(c))
		assert.True(t, v3.ClusterConditionPreBootstrapped.IsTrue(c))
		return c, nil
	})

	_, err := handler(cluster.Name, cluster)
	require.NoError(t, err)
	assert.Empty(t, v3.ClusterConditionControllersRunning.GetStatus(cluster), "the cached object must not be changed")
}

func TestStatusHandlerDoesNotUpdateAnUpToDateStatus(t *testing.T) {
	// Every sync of the cluster runs the handler, so reporting the same state must not write anything.
	u := newStatusTestController(true, clustermanager.ControllersState{Phase: clustermanager.ControllersRunning, Owner: true, PreBootstrapped: true})
	handler, _ := registerTestStatusHandler(t, u)
	cluster := syncTestStatus(t, u, newProvisionedCluster())

	// No UpdateStatus expectation: gomock fails the test if it is called.
	_, err := handler(cluster.Name, cluster)
	require.NoError(t, err)
}
