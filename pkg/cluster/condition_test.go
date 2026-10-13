package cluster

import (
	"errors"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func newConditionTestCluster(uid types.UID) *apimgmtv3.Cluster {
	return &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: uid}}
}

func newConditionTestClient(stored *apimgmtv3.Cluster, conflicts int) *fakes.ClusterInterfaceMock {
	return &fakes.ClusterInterfaceMock{
		GetFunc: func(string, metav1.GetOptions) (*apimgmtv3.Cluster, error) {
			return stored.DeepCopy(), nil
		},
		UpdateStatusFunc: func(c *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
			if conflicts > 0 {
				conflicts--
				return nil, apierrors.NewConflict(schema.GroupResource{Resource: "clusters"}, c.Name, errors.New("modified"))
			}
			stored = c.DeepCopy()
			return c, nil
		},
	}
}

func TestSetConditionUpdatesTheLatestCluster(t *testing.T) {
	stored := newConditionTestCluster("uid-1")
	stored.ResourceVersion = "2"
	client := newConditionTestClient(stored, 0)

	err := SetCondition(client, newConditionTestCluster("uid-1"), apimgmtv3.ClusterConditionUserControllersStopped, corev1.ConditionTrue, "Stopped", "done")

	require.NoError(t, err)
	require.Len(t, client.UpdateStatusCalls(), 1)
	updated := client.UpdateStatusCalls()[0].In1
	assert.Equal(t, "2", updated.ResourceVersion, "the update should be made on the latest version")
	assert.True(t, apimgmtv3.ClusterConditionUserControllersStopped.IsTrue(updated))
	assert.Equal(t, "Stopped", apimgmtv3.ClusterConditionUserControllersStopped.GetReason(updated))
	assert.Equal(t, "done", apimgmtv3.ClusterConditionUserControllersStopped.GetMessage(updated))
}

func TestSetConditionRetriesConflicts(t *testing.T) {
	client := newConditionTestClient(newConditionTestCluster("uid-1"), 2)

	err := SetCondition(client, newConditionTestCluster("uid-1"), apimgmtv3.ClusterConditionUserControllersStopped, corev1.ConditionTrue, "Stopped", "done")

	require.NoError(t, err)
	assert.Len(t, client.UpdateStatusCalls(), 3)
	assert.Len(t, client.GetCalls(), 3, "every attempt should start from a fresh read")
}

func TestSetConditionSkipsAnUnchangedCondition(t *testing.T) {
	stored := newConditionTestCluster("uid-1")
	apimgmtv3.ClusterConditionUserControllersStopped.True(stored)
	apimgmtv3.ClusterConditionUserControllersStopped.Reason(stored, "Stopped")
	apimgmtv3.ClusterConditionUserControllersStopped.Message(stored, "done")
	client := newConditionTestClient(stored, 0)

	err := SetCondition(client, newConditionTestCluster("uid-1"), apimgmtv3.ClusterConditionUserControllersStopped, corev1.ConditionTrue, "Stopped", "done")

	require.NoError(t, err)
	assert.Empty(t, client.UpdateStatusCalls())
}

func TestSetConditionRefusesANewClusterWithTheSameName(t *testing.T) {
	client := newConditionTestClient(newConditionTestCluster("uid-2"), 0)

	err := SetCondition(client, newConditionTestCluster("uid-1"), apimgmtv3.ClusterConditionUserControllersStopped, corev1.ConditionTrue, "Stopped", "done")

	require.Error(t, err)
	assert.Empty(t, client.UpdateStatusCalls())
}

func TestConditionConcluded(t *testing.T) {
	cluster := newConditionTestCluster("uid-1")
	assert.False(t, ConditionConcluded(cluster, apimgmtv3.ClusterConditionAgentUninstallScheduled), "unset")

	apimgmtv3.ClusterConditionAgentUninstallScheduled.Unknown(cluster)
	assert.False(t, ConditionConcluded(cluster, apimgmtv3.ClusterConditionAgentUninstallScheduled), "unknown")

	apimgmtv3.ClusterConditionAgentUninstallScheduled.False(cluster)
	assert.True(t, ConditionConcluded(cluster, apimgmtv3.ClusterConditionAgentUninstallScheduled), "false")

	apimgmtv3.ClusterConditionAgentUninstallScheduled.True(cluster)
	assert.True(t, ConditionConcluded(cluster, apimgmtv3.ClusterConditionAgentUninstallScheduled), "true")
}
