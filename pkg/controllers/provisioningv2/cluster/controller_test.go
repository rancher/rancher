package cluster

import (
	"encoding/json"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
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

func TestGenerateProvisioningClusterFromLegacyCluster(t *testing.T) {
	tests := []struct {
		name    string
		cluster *v3.Cluster
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
				},
			},
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := handler{}

			obj, _, err := h.generateProvisioningClusterFromLegacyCluster(tt.cluster, tt.cluster.Status)

			assert.Nil(t, err)
			assert.NotNil(t, obj, "Expected non-nil prov cluster obj")
			provCluster := obj[0].(*v1.Cluster)

			switch tt.name {
			case "test-default":
				assert.Nil(t, provCluster.Spec.ClusterAgentDeploymentCustomization)
				assert.Nil(t, provCluster.Spec.FleetAgentDeploymentCustomization)
			case "test-cluster-agent-customization":
				assert.Equal(t, getTestClusterAgentCustomizationV1(), provCluster.Spec.ClusterAgentDeploymentCustomization)
				assert.Equal(t, getTestFleetAgentCustomizationV1(), provCluster.Spec.FleetAgentDeploymentCustomization)
			}
		})
	}
}

func defaultClusterSpec() v3.ClusterSpec {
	return v3.ClusterSpec{
		ClusterSpecBase: v3.ClusterSpecBase{
			// default agent image
			DesiredAgentImage: settings.AgentImage.Get(),
		},
	}
}

func TestCreateNewCluster(t *testing.T) {
	tests := []struct {
		name                string
		cluster             *v1.Cluster
		mgmtClusterSpec     v3.ClusterSpec
		expectedClusterSpec v3.ClusterSpec
	}{
		{
			name: "test-default",
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       v1.ClusterSpec{},
			},
			mgmtClusterSpec:     v3.ClusterSpec{},
			expectedClusterSpec: defaultClusterSpec(),
		},
		{
			name: "test-cluster-agent-customization",
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1.ClusterSpec{
					ClusterAgentDeploymentCustomization: getTestClusterAgentCustomizationV1(),
					FleetAgentDeploymentCustomization:   getTestFleetAgentCustomizationV1(),
				},
			},
			expectedClusterSpec: func() v3.ClusterSpec {
				spec := defaultClusterSpec()
				spec.ClusterSpecBase.ClusterAgentDeploymentCustomization = getTestClusterAgentCustomizationV3()
				spec.ClusterSpecBase.FleetAgentDeploymentCustomization = getTestFleetAgentCustomizationV3()
				return spec
			}(),
		},
		{
			name: "test-cluster-desired-image",
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       v1.ClusterSpec{},
			},
			mgmtClusterSpec: v3.ClusterSpec{
				ClusterSpecBase: v3.ClusterSpecBase{
					DesiredAgentImage: "rancher/rancher-agent:test",
					DesiredAuthImage:  "rancher/kube-api-auth:test",
				},
			},
			expectedClusterSpec: v3.ClusterSpec{
				ClusterSpecBase: v3.ClusterSpecBase{
					DesiredAgentImage: "rancher/rancher-agent:test",
					// desired auth image is not set if local auth endpoint is disabled
					DesiredAuthImage: "",
				},
			},
		},
		{
			name: "test-cluster-desired-auth-image",
			cluster: &v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       v1.ClusterSpec{},
			},
			mgmtClusterSpec: v3.ClusterSpec{
				ClusterSpecBase: v3.ClusterSpecBase{
					DesiredAuthImage:         "rancher/kube-api-auth:test",
					LocalClusterAuthEndpoint: v3.LocalClusterAuthEndpoint{Enabled: true},
				},
			},
			expectedClusterSpec: v3.ClusterSpec{
				ClusterSpecBase: v3.ClusterSpecBase{
					DesiredAuthImage: "rancher/kube-api-auth:test",
					// default agent image
					DesiredAgentImage: settings.AgentImage.Get(),
					// prov cluster is source of truth for local cluster auth endpoint
					LocalClusterAuthEndpoint: v3.LocalClusterAuthEndpoint{},
				},
			},
		},
	}

	mockCtrl := gomock.NewController(t)
	clusterCache := fake.NewMockNonNamespacedCacheInterface[*v3.Cluster](mockCtrl)
	clusterCache.EXPECT().Get(gomock.AssignableToTypeOf("")).Return(&v3.Cluster{}, nil).AnyTimes()
	h := handler{
		mgmtClusterCache: clusterCache,
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, _, err := h.createNewCluster(tt.cluster, tt.cluster.Status, tt.mgmtClusterSpec)
			assert.Nil(t, err)
			assert.NotNil(t, obj, "Expected non-nil v3 cluster obj")

			jsonData, _ := json.Marshal(obj[0])
			var legacyCluster v3.Cluster
			json.Unmarshal(jsonData, &legacyCluster)

			assert.Equal(t, tt.expectedClusterSpec, legacyCluster.Spec)
		})
	}
}

func getTestClusterAgentCustomizationV1() *v1.AgentDeploymentCustomization {
	return &v1.AgentDeploymentCustomization{
		AppendTolerations:            getTestClusterAgentToleration(),
		OverrideAffinity:             getTestClusterAgentAffinity(),
		OverrideResourceRequirements: getTestClusterAgentResourceReq(),
	}
}

func getTestClusterAgentCustomizationV3() *v3.AgentDeploymentCustomization {
	return &v3.AgentDeploymentCustomization{
		AppendTolerations:            getTestClusterAgentToleration(),
		OverrideAffinity:             getTestClusterAgentAffinity(),
		OverrideResourceRequirements: getTestClusterAgentResourceReq(),
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
