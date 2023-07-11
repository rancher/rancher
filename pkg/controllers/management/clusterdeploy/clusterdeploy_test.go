package clusterdeploy

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterDeploy_redeployAgent(t *testing.T) {
	testClusterAgentToleration := []corev1.Toleration{{
		Effect: "NoSchedule",
		Key:    "node-role.kubernetes.io/controlplane-test",
		Value:  "true",
	},
	}
	testUpdatedClusterAgentToleration := []corev1.Toleration{{
		Effect: "NoSchedule",
		Key:    "node-role.kubernetes.io/controlplane-test",
		Value:  "true",
	}, {
		Effect: "NoSchedule",
		Key:    "node-role.kubernetes.io/etcd-test",
		Value:  "true",
	},
	}

	tests := []struct {
		name             string
		cluster          *v3.Cluster
		expectedRedeploy bool
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
				Status: v3.ClusterStatus{
					AppliedAgentEnvVars: settings.DefaultAgentSettingsAsEnvVars(),
				},
			},
			expectedRedeploy: false,
		},
		{
			name: "test-add-cluster-agent-customization",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-add-cluster-agent-customization",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						AppendTolerations: testClusterAgentToleration,
					},
				},
			},
			expectedRedeploy: true,
		},
		{
			name: "test-update-cluster-agent-customization",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-default",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							AppendTolerations: testClusterAgentToleration,
						},
					},
				},
				Status: v3.ClusterStatus{
					AppliedClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
						AppendTolerations: testUpdatedClusterAgentToleration,
					},
				},
			},
			expectedRedeploy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v3.ClusterConditionAgentDeployed.Message(tt.cluster, "Successfully rolled out agent")
			v3.ClusterConditionAgentDeployed.True(tt.cluster)

			doRedeploy := redeployAgent(tt.cluster, "", "", nil, nil)
			assert.Equal(t, tt.expectedRedeploy, doRedeploy)
		})
	}
}
