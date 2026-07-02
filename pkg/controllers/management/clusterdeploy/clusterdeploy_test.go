package clusterdeploy

import (
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestDesiredAppliedAgentEnvVars(t *testing.T) {
	cluster := &apimgmtv3.Cluster{
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{{
					Name:  "CUSTOM_VAR",
					Value: "custom",
				}},
			},
		},
	}

	withoutPreBootstrap := desiredAppliedAgentEnvVars(cluster, false)
	withPreBootstrap := desiredAppliedAgentEnvVars(cluster, true)

	assert.True(t, hasEnvVar(withoutPreBootstrap, "CUSTOM_VAR", "custom"))
	assert.False(t, hasEnvVar(withoutPreBootstrap, preBootstrapEnvVarName, "true"))

	assert.True(t, hasEnvVar(withPreBootstrap, "CUSTOM_VAR", "custom"))
	assert.True(t, hasEnvVar(withPreBootstrap, preBootstrapEnvVarName, "true"))
	assert.Equal(t, len(withoutPreBootstrap)+1, len(withPreBootstrap))
}

func TestClusterAgentDeployment(t *testing.T) {
	assert.Equal(t, bootstrapClusterAgentDeploymentName, clusterAgentDeployment(true))
	assert.Equal(t, clusterAgentDeploymentName, clusterAgentDeployment(false))
}

func hasEnvVar(vars []corev1.EnvVar, name, value string) bool {
	for _, envVar := range vars {
		if envVar.Name == name && envVar.Value == value {
			return true
		}
	}

	return false
}
