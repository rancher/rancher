package cluster

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAgentCustomization_getAgentCustomization(t *testing.T) {
	testClusterAgentToleration := []corev1.Toleration{{
		Effect: "NoSchedule",
		Key:    "node-role.kubernetes.io/controlplane-test",
		Value:  "true",
	},
	}
	testClusterAgentAffinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				{
					Weight: 1,
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "cattle.io/cluster-agent-test",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
				},
			},
		},
	}
	testClusterAgentResourceReq := &corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			"cpu":    *resource.NewQuantity(500, resource.DecimalSI),
			"memory": *resource.NewQuantity(250, resource.DecimalSI),
		},
		Requests: map[corev1.ResourceName]resource.Quantity{
			"cpu":    *resource.NewQuantity(500, resource.DecimalSI),
			"memory": *resource.NewQuantity(250, resource.DecimalSI),
		},
	}

	testFleetAgentToleration := []corev1.Toleration{
		{
			Key:      "key",
			Operator: corev1.TolerationOpEqual,
			Value:    "value",
		},
	}
	testFleetAgentAffinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				{
					Weight: 1,
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "fleet.cattle.io/agent",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"true"},
							},
						},
					},
				},
			},
		},
	}
	testFleetAgentResourceReq := &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}

	tests := []struct {
		name    string
		cluster *v3.Cluster
	}{
		{
			name: "test-default",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-default",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
		},
		{
			name: "test-agent-customization",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-agent-customization",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							AppendTolerations:            testClusterAgentToleration,
							OverrideAffinity:             testClusterAgentAffinity,
							OverrideResourceRequirements: testClusterAgentResourceReq,
						},
						FleetAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							AppendTolerations:            testFleetAgentToleration,
							OverrideAffinity:             testFleetAgentAffinity,
							OverrideResourceRequirements: testFleetAgentResourceReq,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterAgentToleration := GetClusterAgentTolerations(tt.cluster)
			clusterAgentAffinity, clusterErr := GetClusterAgentAffinity(tt.cluster)
			clusterAgentResourceRequirements := GetClusterAgentResourceRequirements(tt.cluster)

			fleetAgentToleration := GetFleetAgentTolerations(tt.cluster)
			fleetAgentAffinity, fleetErr := GetFleetAgentAffinity(tt.cluster)
			fleetAgentResourceRequirements := GetFleetAgentResourceRequirements(tt.cluster)

			switch tt.name {
			case "test-default":
				// cluster agent
				assert.Nil(t, clusterAgentToleration)
				defaultClusterAgentAffinity, err := unmarshalAffinity(settings.ClusterAgentDefaultAffinity.Get())
				if err != nil {
					assert.FailNow(t, "failed to unmarshal node affinity: %w", err)
				}
				assert.Equal(t, defaultClusterAgentAffinity, clusterAgentAffinity)
				assert.Nil(t, clusterErr)
				assert.Nil(t, clusterAgentResourceRequirements)

				// fleet agent
				assert.Nil(t, fleetAgentToleration)
				defaultFleetAgentAffinity, err := unmarshalAffinity(settings.FleetAgentDefaultAffinity.Get())
				if err != nil {
					assert.FailNow(t, "failed to unmarshal node affinity: %w", err)
				}
				assert.Equal(t, defaultFleetAgentAffinity, fleetAgentAffinity)
				assert.Nil(t, fleetErr)
				assert.Nil(t, fleetAgentResourceRequirements)
			case "test-agent-customization":
				// cluster agent
				assert.Equal(t, testClusterAgentToleration, clusterAgentToleration)
				assert.Equal(t, testClusterAgentAffinity, clusterAgentAffinity)
				assert.Nil(t, clusterErr)
				assert.Equal(t, testClusterAgentResourceReq, clusterAgentResourceRequirements)

				// fleet agent
				assert.Equal(t, testFleetAgentToleration, fleetAgentToleration)
				assert.Equal(t, testFleetAgentAffinity, fleetAgentAffinity)
				assert.Nil(t, fleetErr)
				assert.Equal(t, testFleetAgentResourceReq, fleetAgentResourceRequirements)
			}
		})
	}

	// Simulate a user setting default affinity as an invalid str
	settings.ClusterAgentDefaultAffinity.Set("test-invalid-affinity")
	settings.FleetAgentDefaultAffinity.Set("test-invalid-affinity")

	// Run tests again and verify that when the cluster agent or fleet agent default affinity is pulled it returns
	// nil and an error.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterAgentAffinity, clusterErr := GetClusterAgentAffinity(tt.cluster)
			fleetAgentAffinity, fleetErr := GetFleetAgentAffinity(tt.cluster)

			switch tt.name {
			case "test-default":
				// cluster agent
				assert.Nil(t, clusterAgentAffinity)
				assert.ErrorContains(t, clusterErr, "failed to unmarshal node affinity")

				// fleet agent
				assert.Nil(t, fleetAgentAffinity)
				assert.ErrorContains(t, fleetErr, "failed to unmarshal node affinity")
			case "test-agent-customization":
				// cluster agent
				assert.Equal(t, testClusterAgentAffinity, clusterAgentAffinity)
				assert.Nil(t, clusterErr)

				// fleet agent
				assert.Equal(t, testFleetAgentAffinity, fleetAgentAffinity)
				assert.Nil(t, fleetErr)
			}
		})
	}
}
