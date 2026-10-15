package cluster

import (
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
)

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
