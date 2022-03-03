package clusterregistrationtoken

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestAgentEnvVars(t *testing.T) {
	c := &v3.Cluster{
		Spec: v3.ClusterSpec{
			ClusterSpecBase: v3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{
						Name:  "HTTPS_PROXY",
						Value: "https://0.0.0.0",
					},
					{
						Name:  "HTTP_PROXY",
						Value: "http://0.0.0.0",
					},
				},
			},
		},
	}
	tests := []struct {
		name     string
		cluster  *v3.Cluster
		envType  EnvType
		expected string
	}{
		{
			name:     "Envvars should be an empty string with a nil cluster",
			cluster:  nil,
			envType:  Linux,
			expected: "",
		},
		{
			name:     "Envvars should be an empty string with no vars set on cluster",
			cluster:  &v3.Cluster{},
			envType:  Linux,
			expected: "",
		},
		{
			name:     "Envvars should be formatted correctly for Linux",
			cluster:  c,
			envType:  Linux,
			expected: "HTTPS_PROXY=\"https://0.0.0.0\" HTTP_PROXY=\"http://0.0.0.0\"",
		},
		{
			name:     "Envvars should be formatted correctly for Docker",
			cluster:  c,
			envType:  Docker,
			expected: "-e \"HTTPS_PROXY=https://0.0.0.0\" -e \"HTTP_PROXY=http://0.0.0.0\"",
		},
		{
			name:     "Envvars should be formatted correctly for PowerShell",
			cluster:  c,
			envType:  PowerShell,
			expected: "$env:HTTPS_PROXY=\"https://0.0.0.0\"; $env:HTTP_PROXY=\"http://0.0.0.0\";",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			a := assert.New(t)

			// act
			evars := AgentEnvVars(tt.cluster, tt.envType)

			// assert
			a.Equal(tt.expected, evars)
		})
	}
}
