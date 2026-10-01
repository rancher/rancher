package usercontrollers

import (
	"errors"
	"testing"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type removeTestEnv struct {
	clusters   *fakes.ClusterInterfaceMock
	controller *fakes.ClusterControllerMock
	stored     *v32.Cluster
}

// newRemoveTestEnv returns a cleanup lifecycle for a cluster of a type that gets no agent uninstall, so
// that Remove never needs to reach the downstream cluster.
func newRemoveTestEnv(stored *v32.Cluster) (*ClusterLifecycleCleanup, *removeTestEnv) {
	env := &removeTestEnv{stored: stored, controller: &fakes.ClusterControllerMock{
		EnqueueAfterFunc: func(string, string, time.Duration) {},
	}}
	env.clusters = &fakes.ClusterInterfaceMock{
		GetFunc: func(string, metav1.GetOptions) (*v32.Cluster, error) {
			return env.stored.DeepCopy(), nil
		},
		UpdateStatusFunc: func(c *v32.Cluster) (*v32.Cluster, error) {
			env.stored = c.DeepCopy()
			return c, nil
		},
		ControllerFunc: func() v3.ClusterController {
			return env.controller
		},
	}
	return &ClusterLifecycleCleanup{Manager: &clustermanager.Manager{}, clusters: env.clusters}, env
}

func newRemoveTestCluster() *v32.Cluster {
	now := metav1.NewTime(time.Now())
	return &v32.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: "uid-1", DeletionTimestamp: &now}}
}

func TestRemoveRecordsTheAgentUninstallAndWaitsForTheControllersToStop(t *testing.T) {
	lifecycle, env := newRemoveTestEnv(newRemoveTestCluster())

	obj, err := lifecycle.Remove(newRemoveTestCluster())

	assert.ErrorIs(t, err, generic.ErrSkip, "the finalizer should be kept")
	assert.NotNil(t, obj)
	assert.True(t, v32.ClusterConditionAgentUninstallScheduled.IsTrue(env.stored))
	assert.Equal(t, reasonNotRequired, v32.ClusterConditionAgentUninstallScheduled.GetReason(env.stored))
	require.Len(t, env.controller.EnqueueAfterCalls(), 1, "the cluster should be checked again")
	assert.Equal(t, "c-m-test", env.controller.EnqueueAfterCalls()[0].Name)
}

func TestRemoveDoesNotScheduleTheAgentUninstallTwice(t *testing.T) {
	cluster := newRemoveTestCluster()
	v32.ClusterConditionAgentUninstallScheduled.False(cluster)
	v32.ClusterConditionAgentUninstallScheduled.Reason(cluster, reasonSchedulingFailed)
	lifecycle, env := newRemoveTestEnv(cluster.DeepCopy())

	_, err := lifecycle.Remove(cluster)

	assert.ErrorIs(t, err, generic.ErrSkip)
	assert.Empty(t, env.clusters.UpdateStatusCalls(), "a recorded outcome should be left alone")
}

func TestRemoveFinishesOnceTheControllersAreStopped(t *testing.T) {
	cluster := newRemoveTestCluster()
	v32.ClusterConditionAgentUninstallScheduled.True(cluster)
	v32.ClusterConditionUserControllersStopped.True(cluster)
	lifecycle, env := newRemoveTestEnv(cluster.DeepCopy())

	obj, err := lifecycle.Remove(cluster)

	require.NoError(t, err, "the finalizer should be removed")
	assert.Nil(t, obj)
	assert.Empty(t, env.clusters.UpdateStatusCalls())
	assert.Empty(t, env.controller.EnqueueAfterCalls())
}

func TestRemoveRetriesWhenTheOutcomeCannotBeRecorded(t *testing.T) {
	lifecycle, env := newRemoveTestEnv(newRemoveTestCluster())
	env.clusters.UpdateStatusFunc = func(*v32.Cluster) (*v32.Cluster, error) {
		return nil, errors.New("boom")
	}

	_, err := lifecycle.Remove(newRemoveTestCluster())

	require.Error(t, err)
	assert.NotErrorIs(t, err, generic.ErrSkip, "a failure should be retried with backoff")
}

func TestRemoveLeavesTheObjectItWasGivenUnchanged(t *testing.T) {
	// The lifecycle persists the returned object with Update, which ignores status on a resource with a
	// status subresource and conflicts with the status update just made. Conditions must only be written
	// through UpdateStatus.
	lifecycle, _ := newRemoveTestEnv(newRemoveTestCluster())
	given := newRemoveTestCluster()
	before := given.DeepCopy()

	_, _ = lifecycle.Remove(given)

	assert.Equal(t, before, given)
}
