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

func TestAgentCustomization_getPriorityClassValueAndPreemption(t *testing.T) {
	neverPreemptionPolicy := corev1.PreemptionPolicy("Never")
	tests := []struct {
		name               string
		cluster            *v3.Cluster
		expectedValue      int
		expectedPreemption string
	}{
		{
			name:               "cluster is nil",
			cluster:            nil,
			expectedValue:      0,
			expectedPreemption: "",
		},
		{
			name: "PC is not configured",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{},
			},
			expectedValue:      0,
			expectedPreemption: "",
		},
		{
			name: "Only PC value is configured",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PriorityClass: &v3.PriorityClassSpec{
									Value: 12345,
								},
							},
						},
					},
				},
			},
			expectedValue:      12345,
			expectedPreemption: "",
		},
		{
			name: "Only PC Preemption is configured",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PriorityClass: &v3.PriorityClassSpec{
									Preemption: &neverPreemptionPolicy,
								},
							},
						},
					},
				},
			},
			expectedValue:      0,
			expectedPreemption: "Never",
		},
		{
			name: "Both fields are configured",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PriorityClass: &v3.PriorityClassSpec{
									Value:      12345,
									Preemption: &neverPreemptionPolicy,
								},
							},
						},
					},
				},
			},
			expectedValue:      12345,
			expectedPreemption: "Never",
		},
	}

	t.Parallel()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			pcValue, preemption := GetDesiredPriorityClassValueAndPreemption(tt.cluster)
			assert.Equal(t, tt.expectedPreemption, preemption)
			assert.Equal(t, tt.expectedValue, pcValue)
		})
	}
}

func TestAgentCustomization_getAgentSchedulingCustomizationStatus(t *testing.T) {
	tests := []struct {
		name           string
		cluster        *v3.Cluster
		expectedStatus *v3.AgentSchedulingCustomization
	}{
		{
			name: "full scheduling customization exists",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						SchedulingCustomization: &v3.AgentSchedulingCustomization{
							PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
								MinAvailable: "1",
							},
							PriorityClass: &v3.PriorityClassSpec{
								Value: 123456,
							},
						},
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MinAvailable: "5",
								},
								PriorityClass: &v3.PriorityClassSpec{
									Value: 654312,
								},
							},
						},
					},
				},
			},
			expectedStatus: &v3.AgentSchedulingCustomization{
				PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
					MinAvailable: "1",
				},
				PriorityClass: &v3.PriorityClassSpec{
					Value: 123456,
				},
			},
		},
		{
			name: "no scheduling customization exists",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{},
				},
			},
			expectedStatus: nil,
		},
		{
			name: "no deployment customization exists",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{},
			},
			expectedStatus: nil,
		},
	}

	t.Parallel()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			returnedStatus := GetAgentSchedulingCustomizationStatus(tt.cluster)
			assert.Equal(t, returnedStatus, tt.expectedStatus)
		})
	}
}

func TestAgentCustomization_agentSchedulingCustomizationEnabled(t *testing.T) {
	tests := []struct {
		name       string
		cluster    *v3.Cluster
		pcEnabled  bool
		pdbEnabled bool
	}{
		{
			name:       "PC and PDB enabled",
			pcEnabled:  true,
			pdbEnabled: true,
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MinAvailable: "1",
								},
								PriorityClass: &v3.PriorityClassSpec{
									Value: 12345,
								},
							},
						},
					},
				},
			},
		},
		{
			name:       "Only PC enabled",
			pcEnabled:  true,
			pdbEnabled: false,
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PriorityClass: &v3.PriorityClassSpec{
									Value: 12345,
								},
							},
						},
					},
				},
			},
		},
		{
			name:       "Only PDB enabled",
			pcEnabled:  false,
			pdbEnabled: true,
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MinAvailable: "1",
								},
							},
						},
					},
				},
			},
		},
		{
			name:       "neither enabled",
			pcEnabled:  false,
			pdbEnabled: false,
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{},
					},
				},
			},
		},
	}

	t.Parallel()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			pcEnabled, pdbEnabled := AgentSchedulingCustomizationEnabled(tt.cluster)
			assert.Equal(t, tt.pcEnabled, pcEnabled)
			assert.Equal(t, tt.pdbEnabled, pdbEnabled)
		})
	}
}

func TestAgentCustomization_getAgentSchedulingCustomizationSpec(t *testing.T) {
	tests := []struct {
		name         string
		cluster      *v3.Cluster
		expectedSpec *v3.AgentSchedulingCustomization
	}{
		{
			name: "full scheduling customization exists",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						SchedulingCustomization: &v3.AgentSchedulingCustomization{
							PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
								MinAvailable: "5",
							},
							PriorityClass: &v3.PriorityClassSpec{
								Value: 654321,
							},
						},
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MinAvailable: "1",
								},
								PriorityClass: &v3.PriorityClassSpec{
									Value: 123456,
								},
							},
						},
					},
				},
			},
			expectedSpec: &v3.AgentSchedulingCustomization{
				PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
					MinAvailable: "1",
				},
				PriorityClass: &v3.PriorityClassSpec{
					Value: 123456,
				},
			},
		},
		{
			name: "no scheduling customization exists",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{},
					},
				},
			},
			expectedSpec: nil,
		},
		{
			name: "no deployment customization exists",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
			expectedSpec: nil,
		},
	}

	t.Parallel()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			returnedStatus := GetAgentSchedulingCustomizationSpec(tt.cluster)
			assert.Equal(t, returnedStatus, tt.expectedSpec)
		})
	}
}

func TestAgentCustomization_updateAppliedAgentDeploymentCustomization(t *testing.T) {
	testClusterAgentToleration := []corev1.Toleration{{
		Effect: "NoSchedule",
		Key:    "node-role.kubernetes.io/controlplane-test",
		Value:  "true",
	}}

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

	neverPreemptionPolicy := corev1.PreemptionPolicy("Never")

	tests := []struct {
		name           string
		cluster        *v3.Cluster
		expectedStatus *v3.AgentDeploymentCustomization
	}{
		{
			name: "set all fields",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							AppendTolerations:            testClusterAgentToleration,
							OverrideAffinity:             testClusterAgentAffinity,
							OverrideResourceRequirements: testClusterAgentResourceReq,
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PriorityClass: &v3.PriorityClassSpec{
									Value:      123456,
									Preemption: &neverPreemptionPolicy,
								},
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MinAvailable: "1",
								},
							},
						},
					},
				},
			},
			expectedStatus: &v3.AgentDeploymentCustomization{
				AppendTolerations:            testClusterAgentToleration,
				OverrideAffinity:             testClusterAgentAffinity,
				OverrideResourceRequirements: testClusterAgentResourceReq,
				SchedulingCustomization: &v3.AgentSchedulingCustomization{
					PriorityClass: &v3.PriorityClassSpec{
						Value:      123456,
						Preemption: &neverPreemptionPolicy,
					},
					PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
						MinAvailable: "1",
					},
				},
			},
		},
		{
			name: "update scheduling fields",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						AppendTolerations:            testClusterAgentToleration,
						OverrideAffinity:             testClusterAgentAffinity,
						OverrideResourceRequirements: testClusterAgentResourceReq,
						SchedulingCustomization: &v3.AgentSchedulingCustomization{
							PriorityClass: &v3.PriorityClassSpec{
								Value:      123456,
								Preemption: &neverPreemptionPolicy,
							},
							PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
								MinAvailable: "1",
							},
						},
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							AppendTolerations:            testClusterAgentToleration,
							OverrideAffinity:             testClusterAgentAffinity,
							OverrideResourceRequirements: testClusterAgentResourceReq,
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PriorityClass: &v3.PriorityClassSpec{
									Value:      654321,
									Preemption: &neverPreemptionPolicy,
								},
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MinAvailable: "2",
								},
							},
						},
					},
				},
			},
			expectedStatus: &v3.AgentDeploymentCustomization{
				AppendTolerations:            testClusterAgentToleration,
				OverrideAffinity:             testClusterAgentAffinity,
				OverrideResourceRequirements: testClusterAgentResourceReq,
				SchedulingCustomization: &v3.AgentSchedulingCustomization{
					PriorityClass: &v3.PriorityClassSpec{
						Value:      654321,
						Preemption: &neverPreemptionPolicy,
					},
					PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
						MinAvailable: "2",
					},
				},
			},
		},
		{
			name: "clear all fields, partial removal of deployment customization",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						AppendTolerations:            testClusterAgentToleration,
						OverrideAffinity:             testClusterAgentAffinity,
						OverrideResourceRequirements: testClusterAgentResourceReq,
						SchedulingCustomization: &v3.AgentSchedulingCustomization{
							PriorityClass: &v3.PriorityClassSpec{
								Value:      123456,
								Preemption: &neverPreemptionPolicy,
							},
							PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
								MinAvailable: "1",
							},
						},
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{},
					},
				},
			},
			expectedStatus: nil,
		},
		{
			name: "clear all fields",
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						AppendTolerations:            testClusterAgentToleration,
						OverrideAffinity:             testClusterAgentAffinity,
						OverrideResourceRequirements: testClusterAgentResourceReq,
						SchedulingCustomization: &v3.AgentSchedulingCustomization{
							PriorityClass: &v3.PriorityClassSpec{
								Value:      123456,
								Preemption: &neverPreemptionPolicy,
							},
							PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
								MinAvailable: "1",
							},
						},
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
			expectedStatus: nil,
		},
	}

	t.Parallel()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			UpdateAppliedAgentDeploymentCustomization(tt.cluster)
			assert.Equal(t, tt.expectedStatus, tt.cluster.Status.AppliedClusterAgentDeploymentCustomization)
		})
	}
}

func TestAgentCustomization_getDesiredPodDisruptionBudgetValuesAsString(t *testing.T) {
	tests := []struct {
		name                   string
		cluster                *v3.Cluster
		expectedMinAvailable   string
		expectedMaxUnavailable string
	}{
		{
			name:                   "nil cluster",
			expectedMaxUnavailable: "",
			expectedMinAvailable:   "",
		},
		{
			name: "no PDB Configured",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{},
			},
			expectedMaxUnavailable: "",
			expectedMinAvailable:   "",
		},
		{
			name: "max unavailable configured",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MaxUnavailable: "1",
								},
							},
						},
					},
				},
			},
			expectedMaxUnavailable: "1",
			expectedMinAvailable:   "",
		},
		{
			name: "min available configured",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MinAvailable: "1",
								},
							},
						},
					},
				},
			},
			expectedMaxUnavailable: "",
			expectedMinAvailable:   "1",
		},
		{
			name: "both values are set to zero ints",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MinAvailable:   "0",
									MaxUnavailable: "0",
								},
							},
						},
					},
				},
			},
			expectedMaxUnavailable: "0",
			expectedMinAvailable:   "",
		},
	}

	t.Parallel()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			minAvailable, maxUnavailable := GetDesiredPodDisruptionBudgetValues(tt.cluster)
			assert.Equal(t, tt.expectedMinAvailable, minAvailable)
			assert.Equal(t, tt.expectedMaxUnavailable, maxUnavailable)
		})
	}
}

func TestAgentCustomization_agentSchedulingPodDisruptionBudgetChanged(t *testing.T) {
	tests := []struct {
		name           string
		cluster        *v3.Cluster
		updateExpected bool
		deleteExpected bool
	}{
		{
			name:           "create new PDB definition",
			updateExpected: true,
			deleteExpected: false,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MinAvailable: "1",
								},
							},
						},
					},
				},
			},
		},
		{
			name:           "only PC definition in spec",
			updateExpected: false,
			deleteExpected: false,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PriorityClass: &v3.PriorityClassSpec{
									Value: 123,
								},
							},
						},
					},
				},
			},
		},
		{
			name:           "only PC definition in status",
			updateExpected: false,
			deleteExpected: false,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						SchedulingCustomization: &v3.AgentSchedulingCustomization{
							PriorityClass: &v3.PriorityClassSpec{
								Value: 123,
							},
						},
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
		},
		{
			name:           "update existing PDB definition",
			updateExpected: true,
			deleteExpected: false,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						SchedulingCustomization: &v3.AgentSchedulingCustomization{
							PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
								MinAvailable: "1",
							},
						},
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MinAvailable: "2",
								},
							},
						},
					},
				},
			},
		},
		{
			name:           "delete existing PDB definition",
			updateExpected: false,
			deleteExpected: true,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						SchedulingCustomization: &v3.AgentSchedulingCustomization{
							PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
								MinAvailable: "1",
							},
						},
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
		},
		{
			name:           "process empty fields",
			updateExpected: false,
			deleteExpected: false,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
		},
		{
			name:           "process empty PDB spec field",
			updateExpected: false,
			deleteExpected: true,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						SchedulingCustomization: &v3.AgentSchedulingCustomization{
							PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
								MinAvailable: "1",
							},
						},
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
		},
		{
			name:           "process empty PDB status field",
			updateExpected: true,
			deleteExpected: false,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MinAvailable: "1",
								},
							},
						},
					},
				},
			},
		},
	}

	t.Parallel()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			pdbUpdated, pdbDeleted := AgentSchedulingPodDisruptionBudgetChanged(tt.cluster)
			assert.Equal(t, tt.updateExpected, pdbUpdated)
			assert.Equal(t, tt.deleteExpected, pdbDeleted)
		})
	}
}

func TestAgentCustomization_agentSchedulingPriorityClassChanged(t *testing.T) {
	tests := []struct {
		name           string
		cluster        *v3.Cluster
		createExpected bool
		updateExpected bool
		deleteExpected bool
	}{
		{
			name:           "create new PC definition",
			createExpected: true,
			updateExpected: false,
			deleteExpected: false,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PriorityClass: &v3.PriorityClassSpec{
									Value: 12345,
								},
							},
						},
					},
				},
			},
		},
		{
			name:           "ignore new PDB definition in spec",
			createExpected: false,
			updateExpected: false,
			deleteExpected: false,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MinAvailable: "1",
								},
							},
						},
					},
				},
			},
		},
		{
			name:           "ignore new PDB definition in status",
			createExpected: false,
			updateExpected: false,
			deleteExpected: false,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						SchedulingCustomization: &v3.AgentSchedulingCustomization{
							PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
								MinAvailable: "1",
							},
						},
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
		},
		{
			name:           "update existing PC definition",
			createExpected: false,
			updateExpected: true,
			deleteExpected: false,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						SchedulingCustomization: &v3.AgentSchedulingCustomization{
							PriorityClass: &v3.PriorityClassSpec{
								Value: 12345,
							},
						},
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PriorityClass: &v3.PriorityClassSpec{
									Value: 54321,
								},
							},
						},
					},
				},
			},
		},
		{
			name:           "delete existing PC definition",
			createExpected: false,
			updateExpected: false,
			deleteExpected: true,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						SchedulingCustomization: &v3.AgentSchedulingCustomization{
							PriorityClass: &v3.PriorityClassSpec{
								Value: 12345,
							},
						},
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
		},
		{
			name:           "no existing PC definition",
			createExpected: false,
			updateExpected: false,
			deleteExpected: false,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
		},
	}

	t.Parallel()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			pcUpdated, pcCreated, pcDeleted := AgentSchedulingPriorityClassChanged(tt.cluster)
			assert.Equal(t, tt.updateExpected, pcUpdated)
			assert.Equal(t, tt.deleteExpected, pcDeleted)
			assert.Equal(t, tt.createExpected, pcCreated)
		})
	}
}

func TestAgentCustomization_agentDeploymentCustomizationChanged(t *testing.T) {
	type test struct {
		name     string
		cluster  *v3.Cluster
		expected bool
	}

	testClusterAgentToleration := []corev1.Toleration{{
		Effect: "NoSchedule",
		Key:    "node-role.kubernetes.io/controlplane-test",
		Value:  "true",
	}}

	modifiedTestClusterAgentToleration := []corev1.Toleration{{
		Effect: "NoSchedule",
		Key:    "node-role.kubernetes.io/controlplane-test",
		Value:  "false",
	}}

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

	modifiedTestClusterAgentAffinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				{
					Weight: 1,
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "cattle.io/cluster-agent-modified",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"false"},
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

	modifiedTestClusterAgentResourceReq := &corev1.ResourceRequirements{
		Limits: map[corev1.ResourceName]resource.Quantity{
			"cpu":    *resource.NewQuantity(5, resource.DecimalSI),
			"memory": *resource.NewQuantity(2, resource.DecimalSI),
		},
		Requests: map[corev1.ResourceName]resource.Quantity{
			"cpu":    *resource.NewQuantity(5, resource.DecimalSI),
			"memory": *resource.NewQuantity(2, resource.DecimalSI),
		},
	}

	tests := []test{
		{
			name:     "No customization",
			expected: false,
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-agent-customization",
				},
				Spec:   v3.ClusterSpec{},
				Status: v3.ClusterStatus{},
			},
		},
		{
			name:     "First time setting customization",
			expected: true,
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
					},
				},
				Status: v3.ClusterStatus{},
			},
		},
		{
			name:     "No changes to existing customization",
			expected: false,
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
					},
				},
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						AppendTolerations:            testClusterAgentToleration,
						OverrideAffinity:             testClusterAgentAffinity,
						OverrideResourceRequirements: testClusterAgentResourceReq,
					},
				},
			},
		},
		{
			name:     "changes to affinity override",
			expected: true,
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-agent-customization",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							OverrideAffinity: modifiedTestClusterAgentAffinity,
						},
					},
				},
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						OverrideAffinity: testClusterAgentAffinity,
					},
				},
			},
		},
		{
			name:     "changes to tolerations",
			expected: true,
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-agent-customization",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							AppendTolerations: modifiedTestClusterAgentToleration,
						},
					},
				},
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						AppendTolerations: testClusterAgentToleration,
					},
				},
			},
		},
		{
			name:     "changes to resource requirements",
			expected: true,
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-agent-customization",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							OverrideResourceRequirements: modifiedTestClusterAgentResourceReq,
						},
					},
				},
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						OverrideResourceRequirements: testClusterAgentResourceReq,
					},
				},
			},
		},
		{
			name:     "changes to all",
			expected: true,
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
					},
				},
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						AppendTolerations:            modifiedTestClusterAgentToleration,
						OverrideAffinity:             modifiedTestClusterAgentAffinity,
						OverrideResourceRequirements: modifiedTestClusterAgentResourceReq,
					},
				},
			},
		},
	}

	t.Parallel()
	for _, tst := range tests {
		tst := tst
		t.Run(tst.name, func(t *testing.T) {
			matches := AgentDeploymentCustomizationChanged(tst.cluster)
			if matches != tst.expected {
				t.Fail()
			}
		})
	}
}

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
