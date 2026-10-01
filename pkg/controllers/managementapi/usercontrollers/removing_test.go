package usercontrollers

import (
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

// fixedOwnerStrategy makes this replica the owner of every cluster, or of none.
type fixedOwnerStrategy struct {
	owner bool
}

func (s fixedOwnerStrategy) isOwner(*v3.Cluster) bool      { return s.owner }
func (s fixedOwnerStrategy) forcedResync() <-chan struct{} { return nil }

func newRemovingCluster(uid types.UID) *v3.Cluster {
	now := metav1.NewTime(time.Now())
	return &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: uid, DeletionTimestamp: &now}}
}

func withAgentUninstall(c *v3.Cluster, scheduled bool) *v3.Cluster {
	if scheduled {
		v3.ClusterConditionAgentUninstallScheduled.True(c)
	} else {
		v3.ClusterConditionAgentUninstallScheduled.False(c)
	}
	return c
}

func TestSyncKeepsControllersRunningUntilTheAgentUninstallIsRecorded(t *testing.T) {
	starter := simpleControllerStarter{}
	controller, _ := newMockUserControllersController(t, &starter)

	_, err := controller.sync("c-m-test", newRemovingCluster("uid-1"))

	require.NoError(t, err)
	assert.False(t, starter.stopCalled, "controllers should keep running until the agent uninstall is recorded")
	assert.False(t, starter.startCalled, "a cluster being removed should never be started")
}

func TestSyncStopsAndReportsOnceTheAgentUninstallIsScheduled(t *testing.T) {
	for _, scheduled := range []bool{true, false} {
		t.Run(map[bool]string{true: "scheduled", false: "scheduling failed"}[scheduled], func(t *testing.T) {
			starter := simpleControllerStarter{}
			controller, client := newMockUserControllersController(t, &starter)
			controller.identity = "rancher-0"
			cluster := withAgentUninstall(newRemovingCluster("uid-1"), scheduled)

			client.EXPECT().Get("c-m-test", gomock.Any()).Return(cluster.DeepCopy(), nil)
			var reported *v3.Cluster
			client.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(c *v3.Cluster) (*v3.Cluster, error) {
				reported = c
				return c, nil
			})

			_, err := controller.sync("c-m-test", cluster)

			require.NoError(t, err)
			require.Len(t, starter.stopped, 1)
			assert.Equal(t, types.UID("uid-1"), starter.stopped[0].UID)
			require.NotNil(t, reported)
			assert.True(t, v3.ClusterConditionUserControllersStopped.IsTrue(reported))
			assert.Equal(t, reasonStopped, v3.ClusterConditionUserControllersStopped.GetReason(reported))
			assert.Contains(t, v3.ClusterConditionUserControllersStopped.GetMessage(reported), "rancher-0")
			assert.False(t, starter.startCalled)
		})
	}
}

func TestSyncStopsButDoesNotReportWhenNotTheOwner(t *testing.T) {
	starter := simpleControllerStarter{}
	controller, _ := newMockUserControllersController(t, &starter)
	controller.ownerStrategy = fixedOwnerStrategy{owner: false}

	_, err := controller.sync("c-m-test", withAgentUninstall(newRemovingCluster("uid-1"), true))

	require.NoError(t, err)
	assert.True(t, starter.stopCalled, "every replica stops its own controllers")
	// The mock client fails the test on any call, so nothing was reported.
}

func TestSyncDoesNotReportAgainOnceReported(t *testing.T) {
	starter := simpleControllerStarter{}
	controller, _ := newMockUserControllersController(t, &starter)
	cluster := withAgentUninstall(newRemovingCluster("uid-1"), true)
	v3.ClusterConditionUserControllersStopped.True(cluster)

	_, err := controller.sync("c-m-test", cluster)

	require.NoError(t, err)
	assert.True(t, starter.stopCalled)
}

func TestSyncIgnoresAClusterThatIsGone(t *testing.T) {
	starter := simpleControllerStarter{}
	controller, _ := newMockUserControllersController(t, &starter)

	_, err := controller.sync("c-m-test", nil)

	require.NoError(t, err)
	assert.False(t, starter.stopCalled)
	assert.False(t, starter.startCalled)
}

func TestOnClusterDeleteStopsTheDeletedCluster(t *testing.T) {
	starter := simpleControllerStarter{}
	controller, _ := newMockUserControllersController(t, &starter)
	deleted := newRemovingCluster("uid-1")

	controller.onClusterDelete(deleted)
	controller.onClusterDelete(cache.DeletedFinalStateUnknown{Key: "c-m-test", Obj: deleted})
	controller.onClusterDelete(cache.DeletedFinalStateUnknown{Key: "c-m-test", Obj: "not a cluster"})

	require.Len(t, starter.stopped, 2, "the object and the tombstone should both stop the cluster")
	assert.Same(t, deleted, starter.stopped[0])
	assert.Same(t, deleted, starter.stopped[1])
}

func TestOnClusterUpdateStopsAClusterReplacedUnderTheSameName(t *testing.T) {
	starter := simpleControllerStarter{}
	controller, _ := newMockUserControllersController(t, &starter)
	old := newRemovingCluster("uid-1")
	replacement := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-2"}}

	controller.onClusterUpdate(old, replacement)

	require.Len(t, starter.stopped, 1)
	assert.Same(t, old, starter.stopped[0], "the replaced cluster should be stopped, not the new one")
}

func TestOnClusterUpdateIgnoresOrdinaryUpdates(t *testing.T) {
	starter := simpleControllerStarter{}
	controller, _ := newMockUserControllersController(t, &starter)
	before := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-1", ResourceVersion: "1"}}
	after := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-1", ResourceVersion: "2"}}

	controller.onClusterUpdate(before, after)

	assert.False(t, starter.stopCalled)
}
