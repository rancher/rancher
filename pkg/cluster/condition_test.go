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

	err := SetCondition(client, newConditionTestCluster("uid-1"), apimgmtv3.ClusterConditionControllersStopped, corev1.ConditionTrue, "Stopped", "done")

	require.NoError(t, err)
	require.Len(t, client.UpdateStatusCalls(), 1)
	updated := client.UpdateStatusCalls()[0].In1
	assert.Equal(t, "2", updated.ResourceVersion, "the update should be made on the latest version")
	assert.True(t, apimgmtv3.ClusterConditionControllersStopped.IsTrue(updated))
	assert.Equal(t, "Stopped", apimgmtv3.ClusterConditionControllersStopped.GetReason(updated))
	assert.Equal(t, "done", apimgmtv3.ClusterConditionControllersStopped.GetMessage(updated))
}

func TestSetConditionRetriesConflicts(t *testing.T) {
	client := newConditionTestClient(newConditionTestCluster("uid-1"), 2)

	err := SetCondition(client, newConditionTestCluster("uid-1"), apimgmtv3.ClusterConditionControllersStopped, corev1.ConditionTrue, "Stopped", "done")

	require.NoError(t, err)
	assert.Len(t, client.UpdateStatusCalls(), 3)
	assert.Len(t, client.GetCalls(), 3, "every attempt should start from a fresh read")
}

func TestSetConditionSkipsAnUnchangedCondition(t *testing.T) {
	stored := newConditionTestCluster("uid-1")
	apimgmtv3.ClusterConditionControllersStopped.True(stored)
	apimgmtv3.ClusterConditionControllersStopped.Reason(stored, "Stopped")
	apimgmtv3.ClusterConditionControllersStopped.Message(stored, "done")
	client := newConditionTestClient(stored, 0)

	err := SetCondition(client, newConditionTestCluster("uid-1"), apimgmtv3.ClusterConditionControllersStopped, corev1.ConditionTrue, "Stopped", "done")

	require.NoError(t, err)
	assert.Empty(t, client.UpdateStatusCalls())
}

func TestSetConditionRefusesANewClusterWithTheSameName(t *testing.T) {
	client := newConditionTestClient(newConditionTestCluster("uid-2"), 0)

	err := SetCondition(client, newConditionTestCluster("uid-1"), apimgmtv3.ClusterConditionControllersStopped, corev1.ConditionTrue, "Stopped", "done")

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

func TestSummarizeCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cluster  *apimgmtv3.Cluster
		expected *apimgmtv3.Condition
	}{
		{
			name: "nil cluster",
		},
		{
			name: "empty cluster",
		},
		{
			name: "single condition true",
			cluster: &apimgmtv3.Cluster{
				Status: apimgmtv3.ClusterStatus{
					Conditions: []apimgmtv3.ClusterCondition{
						{
							Type:    "Connected",
							Reason:  "Connected",
							Message: "Cluster agent connected",
							Status:  "True",
						},
					},
				},
			},
			expected: &apimgmtv3.Condition{
				Type:   "Ready",
				Reason: "ClusterReady",
				Status: "True",
			},
		},
		{
			name: "single condition false",
			cluster: &apimgmtv3.Cluster{
				Status: apimgmtv3.ClusterStatus{
					Conditions: []apimgmtv3.ClusterCondition{
						{
							Type:    "Connected",
							Reason:  "Disconnected",
							Message: "Cluster agent disconnected",
							Status:  "False",
						},
					},
				},
			},
			expected: &apimgmtv3.Condition{
				Type:    "Ready",
				Reason:  "ClusterNotReady",
				Message: "Cluster agent disconnected",
				Status:  "False",
			},
		},
		{
			name: "single condition unknown",
			cluster: &apimgmtv3.Cluster{
				Status: apimgmtv3.ClusterStatus{
					Conditions: []apimgmtv3.ClusterCondition{
						{
							Type:    "Connected",
							Reason:  "Connecting",
							Message: "Cluster agent connecting",
							Status:  "Unknown",
						},
					},
				},
			},
			expected: &apimgmtv3.Condition{
				Type:    "Ready",
				Reason:  "ClusterProgressing",
				Message: "Cluster agent connecting",
				Status:  "Unknown",
			},
		},
		{
			name: "true & true",
			cluster: &apimgmtv3.Cluster{
				Status: apimgmtv3.ClusterStatus{
					Conditions: []apimgmtv3.ClusterCondition{
						{
							Type:    "Connected",
							Reason:  "Connected",
							Message: "Cluster agent connected",
							Status:  "True",
						},
						{
							Type:    "PreBootstrapped",
							Reason:  "PreBootstrappedReason",
							Message: "Cluster prebootstrapped",
							Status:  "True",
						},
					},
				},
			},
			expected: &apimgmtv3.Condition{
				Type:   "Ready",
				Reason: "ClusterReady",
				Status: "True",
			},
		},
		{
			name: "true & unknown",
			cluster: &apimgmtv3.Cluster{
				Status: apimgmtv3.ClusterStatus{
					Conditions: []apimgmtv3.ClusterCondition{
						{
							Type:    "Connected",
							Reason:  "Connected",
							Message: "Cluster agent connected",
							Status:  "True",
						},
						{
							Type:    "PreBootstrapped",
							Reason:  "PreBootstrappingReason",
							Message: "Cluster prebootstrapping",
							Status:  "Unknown",
						},
					},
				},
			},
			expected: &apimgmtv3.Condition{
				Type:    "Ready",
				Reason:  "ClusterProgressing",
				Message: "Cluster prebootstrapping",
				Status:  "Unknown",
			},
		},
		{
			name: "true & false",
			cluster: &apimgmtv3.Cluster{
				Status: apimgmtv3.ClusterStatus{
					Conditions: []apimgmtv3.ClusterCondition{
						{
							Type:    "Connected",
							Reason:  "Connected",
							Message: "Cluster agent connected",
							Status:  "True",
						},
						{
							Type:    "PreBootstrapped",
							Reason:  "PreBootstrappingFailedReason",
							Message: "Cluster prebootstrap failed",
							Status:  "False",
						},
					},
				},
			},
			expected: &apimgmtv3.Condition{
				Type:    "Ready",
				Reason:  "ClusterNotReady",
				Message: "Cluster prebootstrap failed",
				Status:  "False",
			},
		},
		{
			name: "unknown & unknown",
			cluster: &apimgmtv3.Cluster{
				Status: apimgmtv3.ClusterStatus{
					Conditions: []apimgmtv3.ClusterCondition{
						{
							Type:    "Connected",
							Reason:  "Connecting",
							Message: "Cluster agent connecting",
							Status:  "Unknown",
						},
						{
							Type:    "PreBootstrapped",
							Reason:  "PreBootstrappingReason",
							Message: "Cluster prebootstrapping",
							Status:  "Unknown",
						},
					},
				},
			},
			expected: &apimgmtv3.Condition{
				Type:    "Ready",
				Reason:  "ClusterProgressing",
				Message: "Cluster prebootstrapping; Cluster agent connecting", // priority order
				Status:  "Unknown",
			},
		},
		{
			name: "unknown & false",
			cluster: &apimgmtv3.Cluster{
				Status: apimgmtv3.ClusterStatus{
					Conditions: []apimgmtv3.ClusterCondition{
						{
							Type:    "Connected",
							Reason:  "Connecting",
							Message: "Cluster agent connecting",
							Status:  "Unknown",
						},
						{
							Type:    "PreBootstrapped",
							Reason:  "PreBootstrappingFailedReason",
							Message: "Cluster prebootstrap failed",
							Status:  "False",
						},
					},
				},
			},
			expected: &apimgmtv3.Condition{
				Type:    "Ready",
				Reason:  "ClusterNotReady",
				Message: "Cluster prebootstrap failed",
				Status:  "False",
			},
		},
		{
			name: "false & false",
			cluster: &apimgmtv3.Cluster{
				Status: apimgmtv3.ClusterStatus{
					Conditions: []apimgmtv3.ClusterCondition{
						{
							Type:    "Connected",
							Reason:  "Disconnected",
							Message: "Cluster agent disconnected",
							Status:  "False",
						},
						{
							Type:    "PreBootstrapped",
							Reason:  "PreBootstrappingFailedReason",
							Message: "Cluster prebootstrap failed",
							Status:  "False",
						},
					},
				},
			},
			expected: &apimgmtv3.Condition{
				Type:    "Ready",
				Reason:  "ClusterNotReady",
				Message: "Cluster prebootstrap failed; Cluster agent disconnected",
				Status:  "False",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SummarizeCondition(tt.cluster)
			if tt.cluster == nil {
				assert.Nil(t, tt.expected)
			} else if tt.expected == nil {
				for _, condition := range tt.cluster.Status.Conditions {
					assert.NotEqual(t, "Ready", condition.Type, "")
				}
			} else {
				for _, condition := range tt.cluster.Status.Conditions {
					if condition.Type != "Ready" {
						continue
					}
					assert.Equal(t, tt.expected.Status, condition.Status)
					assert.Equal(t, tt.expected.Message, condition.Message)
					assert.Equal(t, tt.expected.Reason, condition.Reason)
				}
			}
		})
	}
}
