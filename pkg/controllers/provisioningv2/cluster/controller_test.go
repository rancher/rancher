package cluster

import (
	"encoding/json"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRegexp(t *testing.T) {
	assert.True(t, mgmtNameRegexp.MatchString("local"))
	assert.False(t, mgmtNameRegexp.MatchString("alocal"))
	assert.False(t, mgmtNameRegexp.MatchString("localb"))
	assert.True(t, mgmtNameRegexp.MatchString("c-12345"))
	assert.False(t, mgmtNameRegexp.MatchString("ac-12345"))
	assert.False(t, mgmtNameRegexp.MatchString("c-12345b"))
	assert.False(t, mgmtNameRegexp.MatchString("ac-12345b"))
}

func TestController_updateImportedCluster(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *v1.Cluster
		mgmtCluster *v3.Cluster
	}{
		{
			name: "test-override",
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"foo": "bar"},
					Annotations: map[string]string{"test.io": "true"},
				},
				Spec: v1.ClusterSpec{
					ClusterAgentDeploymentCustomization: getTestClusterAgentCustomizationV1(),
				},
			},
			mgmtCluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"foo": "bar2"},
					Annotations: map[string]string{"test.io": "false"},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: getTestClusterAgentCustomizationV3Updated(),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			clusterCache := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](mockCtrl)
			clusterCache.EXPECT().Get(gomock.AssignableToTypeOf("")).Return(tt.mgmtCluster, nil).AnyTimes()

			h := handler{
				mgmtClusterCache: clusterCache,
			}

			obj, _, err := h.updateImportedCluster(tt.cluster, tt.cluster.Status, tt.mgmtCluster)

			assert.Nil(t, err)

			jsonData, _ := json.Marshal(obj[0])
			var legacyCluster v3.Cluster
			json.Unmarshal(jsonData, &legacyCluster)

			switch tt.name {
			case "test-override":
				assert.Equal(t, tt.mgmtCluster.Labels, legacyCluster.Labels)
				assert.Equal(t, tt.mgmtCluster.Annotations, legacyCluster.Annotations)
				assert.Equal(t, tt.mgmtCluster.Spec.ClusterAgentDeploymentCustomization, legacyCluster.Spec.ClusterAgentDeploymentCustomization)
			}
		})
	}
}

func TestController_generateProvisioningClusterFromLegacyCluster(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *v3.Cluster
		expected bool
	}{
		{
			name: "test-default",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-11111",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase:    v3.ClusterSpecBase{},
					FleetWorkspaceName: "test-fleet-workspace-name",
					DisplayName:        "test-cluster",
				},
			},
			expected: true,
		},
		{
			name: "test-cluster-agent-customization",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-22222",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: getTestClusterAgentCustomizationV3(),
						FleetAgentDeploymentCustomization:   getTestFleetAgentCustomizationV3(),
					},
					FleetWorkspaceName: "test-fleet-workspace-name",
				},
			},
			expected: true,
		},
		{
			name: "test-turtles-owned-cluster-creation",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-33333",
					Annotations: map[string]string{
						externallyManagedAnn: "true",
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
					// Empty fleet workspace, typically prevents provisioningv1 cluster creation
					FleetWorkspaceName: "",
				},
			},
			expected: true,
		},
		{
			name: "no-cluster-for-no-fleet",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-44444",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase:    v3.ClusterSpecBase{},
					FleetWorkspaceName: "",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := handler{}

			obj, _, err := h.generateProvisioningClusterFromLegacyCluster(tt.cluster, tt.cluster.Status)

			assert.Nil(t, err)

			if !tt.expected {
				assert.Nil(t, obj, "Expected no prov cluster objects")
				return
			}

			assert.NotNil(t, obj, "Expected non-nil prov cluster obj")
			provCluster := obj[0].(*v1.Cluster)

			switch tt.name {
			case "test-default":
				assert.Nil(t, provCluster.Spec.ClusterAgentDeploymentCustomization)
				assert.Nil(t, provCluster.Spec.FleetAgentDeploymentCustomization)
				assert.Equal(t, provCluster.Annotations[mgmtClusterDisplayNameAnn], tt.cluster.Spec.DisplayName)
			case "test-cluster-agent-customization":
				assert.Equal(t, getTestClusterAgentCustomizationV1(), provCluster.Spec.ClusterAgentDeploymentCustomization)
				assert.Equal(t, getTestFleetAgentCustomizationV1(), provCluster.Spec.FleetAgentDeploymentCustomization)
			}
		})
	}
}

func TestController_createNewCluster(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *v1.Cluster
		clusterSpec v3.ClusterSpec
	}{
		{
			name: "test-default",
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       v1.ClusterSpec{},
			},
			clusterSpec: v3.ClusterSpec{},
		},
		{
			name: "test-cluster-agent-customization",
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1.ClusterSpec{
					RKEConfig:                           &v1.RKEConfig{},
					ClusterAgentDeploymentCustomization: getTestClusterAgentCustomizationV1(),
					FleetAgentDeploymentCustomization:   getTestFleetAgentCustomizationV1(),
				},
			},
			clusterSpec: v3.ClusterSpec{
				ClusterSpecBase: v3.ClusterSpecBase{},
			},
		},
	}

	mockCtrl := gomock.NewController(t)
	clusterCache := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](mockCtrl)
	clusterCache.EXPECT().Get(gomock.AssignableToTypeOf("")).Return(&v3.Cluster{}, nil).AnyTimes()
	featureCache := fake.NewMockNonNamespacedCacheInterface[*v3.Feature](mockCtrl)
	rkeLockedValue := true
	featureCache.EXPECT().Get(gomock.AssignableToTypeOf("")).Return(&v3.Feature{
		ObjectMeta: metav1.ObjectMeta{Name: "rke2"},
		Status:     v3.FeatureStatus{LockedValue: &rkeLockedValue},
	}, nil).AnyTimes()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := handler{
				mgmtClusterCache: clusterCache,
				featureCache:     featureCache,
			}

			obj, _, err := h.createNewCluster(tt.cluster, tt.cluster.Status, tt.clusterSpec)

			assert.Nil(t, err)
			assert.NotNil(t, obj, "Expected non-nil v3 cluster obj")
			jsonData, _ := json.Marshal(obj[0])
			var legacyCluster v3.Cluster
			json.Unmarshal(jsonData, &legacyCluster)

			switch tt.name {
			case "test-default":
				assert.Nil(t, legacyCluster.Spec.ClusterAgentDeploymentCustomization)
				assert.Nil(t, legacyCluster.Spec.FleetAgentDeploymentCustomization)
			case "test-cluster-agent-customization":
				assert.Equal(t, getTestClusterAgentCustomizationV3(), legacyCluster.Spec.ClusterAgentDeploymentCustomization)
				assert.Equal(t, getTestFleetAgentCustomizationV3(), legacyCluster.Spec.FleetAgentDeploymentCustomization)
			}
		})
	}
}

func Test_byCloudCredentialIndex(t *testing.T) {
	tests := []struct {
		name    string
		cluster v1.Cluster
		want    []string
	}{
		{
			name:    "no credentials anywhere returns nil",
			cluster: v1.Cluster{},
			want:    nil,
		},
		{
			name: "cluster-level only",
			cluster: v1.Cluster{
				Spec: v1.ClusterSpec{
					CloudCredentialSecretName: "cattle-global-data:cc-aaa",
				},
			},
			want: []string{"cattle-global-data:cc-aaa"},
		},
		{
			name: "pool-level only returns nil",
			cluster: v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						MachinePools: []v1.RKEMachinePool{
							{
								Name: "pool-1",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-bbb",
								},
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "both cluster and pool",
			cluster: v1.Cluster{
				Spec: v1.ClusterSpec{
					CloudCredentialSecretName: "cattle-global-data:cc-ccc",
					RKEConfig: &v1.RKEConfig{
						MachinePools: []v1.RKEMachinePool{
							{
								Name: "pool-a",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-ccc",
								},
							},
						},
					},
				},
			},
			want: []string{"cattle-global-data:cc-ccc"},
		},
		{
			name: "multiple pools but empty cluster-level return nil",
			cluster: v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						MachinePools: []v1.RKEMachinePool{
							{
								Name: "pool-a",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-111",
								},
							},
							{
								Name: "pool-b",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "",
								},
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "cluster-level set alongside pools returns only cluster-level",
			cluster: v1.Cluster{
				Spec: v1.ClusterSpec{
					CloudCredentialSecretName: "cattle-global-data:cc-xyz",
					RKEConfig: &v1.RKEConfig{
						MachinePools: []v1.RKEMachinePool{
							{
								Name: "p1",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-fgh",
								},
							},
							{
								Name: "p2",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-abc",
								},
							},
						},
					},
				},
			},
			want: []string{"cattle-global-data:cc-xyz"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := byCloudCredentialIndex(&tc.cluster)
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.want, got)
		})
	}
}

func Test_byMachinePoolCloudCredIndex(t *testing.T) {
	tests := []struct {
		name    string
		cluster v1.Cluster
		want    []string
	}{
		{
			name:    "no pools returns nil",
			cluster: v1.Cluster{},
			want:    nil,
		},
		{
			name: "single pool credential",
			cluster: v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						MachinePools: []v1.RKEMachinePool{
							{
								Name: "pool-1",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-bbb",
								},
							},
						},
					},
				},
			},
			want: []string{"cattle-global-data:cc-bbb"},
		},
		{
			name: "multiple pools mixed",
			cluster: v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						MachinePools: []v1.RKEMachinePool{
							{
								Name: "pool-a",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-111",
								},
							},
							{
								Name: "pool-b",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "",
								},
							},
							{
								Name: "pool-c",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-222",
								},
							},
						},
					},
				},
			},
			want: []string{"cattle-global-data:cc-111", "cattle-global-data:cc-222"},
		},
		{
			name: "multiple pools ensure deterministic order",
			cluster: v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						MachinePools: []v1.RKEMachinePool{
							{
								Name: "pool-a",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-111",
								},
							},
							{
								Name: "pool-b",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-333",
								},
							},
							{
								Name: "pool-c",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-222",
								},
							},

							{
								Name: "pool-b",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-444",
								},
							},
						},
					},
				},
			},
			want: []string{"cattle-global-data:cc-111", "cattle-global-data:cc-222", "cattle-global-data:cc-333", "cattle-global-data:cc-444"},
		},
		{
			name: "multiple pools with the same credential",
			cluster: v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						MachinePools: []v1.RKEMachinePool{
							{
								Name: "pool-a",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-111",
								},
							},
							{
								Name: "pool-b",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "",
								},
							},
							{
								Name: "pool-c",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-111",
								},
							},
						},
					},
				},
			},
			want: []string{"cattle-global-data:cc-111"},
		},
		{
			name: "cluster-level set but ignored",
			cluster: v1.Cluster{
				Spec: v1.ClusterSpec{
					CloudCredentialSecretName: "cattle-global-data:cc-fgh",
					RKEConfig: &v1.RKEConfig{
						MachinePools: []v1.RKEMachinePool{
							{
								Name: "p1",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-xyz",
								},
							},
							{
								Name: "p2",
								RKECommonNodeConfig: rkev1.RKECommonNodeConfig{
									CloudCredentialSecretName: "cattle-global-data:cc-abc",
								},
							},
						},
					},
				},
			},
			want: []string{"cattle-global-data:cc-abc", "cattle-global-data:cc-xyz"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := byMachinePoolCloudCredIndex(&tc.cluster)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func getTestClusterAgentCustomizationV1() *v1.AgentDeploymentCustomization {
	return &v1.AgentDeploymentCustomization{
		AppendTolerations:            getTestClusterAgentToleration(),
		OverrideAffinity:             getTestClusterAgentAffinity(),
		OverrideResourceRequirements: getTestClusterAgentResourceReq(),
		SchedulingCustomization: &v1.AgentSchedulingCustomization{
			PriorityClass: &v1.PriorityClassSpec{
				Value: 1000,
			},
			PodDisruptionBudget: &v1.PodDisruptionBudgetSpec{
				MinAvailable: "1",
			},
		},
	}
}

func getTestClusterAgentCustomizationV3() *v3.AgentDeploymentCustomization {
	return &v3.AgentDeploymentCustomization{
		AppendTolerations:            getTestClusterAgentToleration(),
		OverrideAffinity:             getTestClusterAgentAffinity(),
		OverrideResourceRequirements: getTestClusterAgentResourceReq(),
		SchedulingCustomization: &v3.AgentSchedulingCustomization{
			PriorityClass: &v3.PriorityClassSpec{
				Value: 1000,
			},
			PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
				MinAvailable: "1",
			},
		},
	}
}

func getTestClusterAgentCustomizationV3Updated() *v3.AgentDeploymentCustomization {
	resourceRequirements := &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("700"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("250Mi"),
		},
	}
	return &v3.AgentDeploymentCustomization{
		AppendTolerations:            getTestClusterAgentToleration(),
		OverrideAffinity:             getTestClusterAgentAffinity(),
		OverrideResourceRequirements: resourceRequirements,
	}
}

func getTestFleetAgentCustomizationV1() *v1.AgentDeploymentCustomization {
	return &v1.AgentDeploymentCustomization{
		AppendTolerations:            getTestFleetAgentToleration(),
		OverrideAffinity:             getTestFleetAgentAffinity(),
		OverrideResourceRequirements: getTestFleetAgentResourceReq(),
	}
}

func getTestFleetAgentCustomizationV3() *v3.AgentDeploymentCustomization {
	return &v3.AgentDeploymentCustomization{
		AppendTolerations:            getTestFleetAgentToleration(),
		OverrideAffinity:             getTestFleetAgentAffinity(),
		OverrideResourceRequirements: getTestFleetAgentResourceReq(),
	}
}

func getTestClusterAgentToleration() []corev1.Toleration {
	return []corev1.Toleration{{
		Effect: "NoSchedule",
		Key:    "node-role.kubernetes.io/controlplane-test",
		Value:  "true",
	},
	}
}

func getTestClusterAgentAffinity() *corev1.Affinity {
	return &corev1.Affinity{
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
}

func getTestClusterAgentResourceReq() *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("500m"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("250Mi"),
		},
	}
}

func getTestFleetAgentToleration() []corev1.Toleration {
	return []corev1.Toleration{
		{
			Key:      "key",
			Operator: corev1.TolerationOpEqual,
			Value:    "value",
		},
	}
}

func getTestFleetAgentAffinity() *corev1.Affinity {
	return &corev1.Affinity{
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
}

func getTestFleetAgentResourceReq() *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("1"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
}
