package provisioningcluster

import (
	"encoding/json"
	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPopulateHostnameLengthLimitAnnotation(t *testing.T) {
	tests := []struct {
		name                       string
		machinePool                provv1.RKEMachinePool
		defaultHostnameLengthLimit int
		expected                   map[string]string
	}{
		{
			name:     "default",
			expected: map[string]string{},
		},
		{
			name:        "machine pool valid",
			machinePool: provv1.RKEMachinePool{HostnameLengthLimit: 32},
			expected:    map[string]string{"rke.cattle.io/hostname-length-limit": "32"},
		},
		{
			name:        "machine pool valid min",
			machinePool: provv1.RKEMachinePool{HostnameLengthLimit: 10},
			expected:    map[string]string{"rke.cattle.io/hostname-length-limit": "10"},
		},
		{
			name:        "machine pool valid max",
			machinePool: provv1.RKEMachinePool{HostnameLengthLimit: 63},
			expected:    map[string]string{"rke.cattle.io/hostname-length-limit": "63"},
		},
		{
			name:        "machine pool < min",
			machinePool: provv1.RKEMachinePool{HostnameLengthLimit: 1},
			expected:    map[string]string{},
		},
		{
			name:        "machine pool > max",
			machinePool: provv1.RKEMachinePool{HostnameLengthLimit: 64},
			expected:    map[string]string{},
		},
		{
			name:                       "default valid",
			defaultHostnameLengthLimit: 32,
			expected:                   map[string]string{"rke.cattle.io/hostname-length-limit": "32"},
		},
		{
			name:                       "default valid min",
			defaultHostnameLengthLimit: 10,
			expected:                   map[string]string{"rke.cattle.io/hostname-length-limit": "10"},
		},
		{
			name:                       "default valid max",
			defaultHostnameLengthLimit: 63,
			expected:                   map[string]string{"rke.cattle.io/hostname-length-limit": "63"},
		},
		{
			name:                       "default < min",
			defaultHostnameLengthLimit: 1,
			expected:                   map[string]string{},
		},
		{
			name:                       "default > max",
			defaultHostnameLengthLimit: 64,
			expected:                   map[string]string{},
		},
		{
			name:                       "prefer pool value over default",
			machinePool:                provv1.RKEMachinePool{HostnameLengthLimit: 16},
			defaultHostnameLengthLimit: 32,
			expected:                   map[string]string{"rke.cattle.io/hostname-length-limit": "16"},
		},
		{
			name:                       "fallback default",
			machinePool:                provv1.RKEMachinePool{HostnameLengthLimit: 1234},
			defaultHostnameLengthLimit: 32,
			expected:                   map[string]string{"rke.cattle.io/hostname-length-limit": "32"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := map[string]string{}
			tt.machinePool.Name = tt.name
			err := populateHostnameLengthLimitAnnotation(tt.machinePool, &provv1.Cluster{Spec: provv1.ClusterSpec{RKEConfig: &provv1.RKEConfig{
				MachinePoolDefaults: provv1.RKEMachinePoolDefaults{HostnameLengthLimit: tt.defaultHostnameLengthLimit},
			}}}, annotations)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, annotations)
		})
	}
}

// TestRKEControlPlaneChartValuesNulls asserts that the RKEControlPlane template
// carries chart values through verbatim, including an explicit null.
//
// A null is how a user removes a chart default, and it used to be lost on the
// way to the generated HelmChartConfig (#56335). The loss was in the merge patch
// apply sends, not here -- this pins the hop before it, so that a regression in
// the template is not mistaken for the patching bug the apply side now fixes
// with WithNullSafePatch.
func TestRKEControlPlaneChartValuesNulls(t *testing.T) {
	tests := []struct {
		name        string
		chartValues map[string]any
		expected    string
	}{
		{
			name:        "nil chart values",
			chartValues: nil,
			expected:    `null`,
		},
		{
			name:        "empty chart values",
			chartValues: map[string]any{},
			expected:    `{}`,
		},
		{
			name: "chart values are carried",
			chartValues: map[string]any{
				"rke2-coredns": map[string]any{"replicas": 2},
			},
			expected: `{"rke2-coredns":{"replicas":2}}`,
		},
		{
			name: "an explicit null is carried",
			chartValues: map[string]any{
				"rke2-coredns": map[string]any{
					"resources": map[string]any{
						"limits": map[string]any{"cpu": nil, "memory": "130Mi"},
					},
				},
			},
			expected: `{"rke2-coredns":{"resources":{"limits":{"cpu":null,"memory":"130Mi"}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := &provv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "fleet-default"},
				Spec: provv1.ClusterSpec{
					RKEConfig: &provv1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							ChartValues: rkev1.GenericMap{Data: tt.chartValues},
						},
					},
				},
			}

			cp, err := rkeControlPlane(cluster)
			require.NoError(t, err)

			// Marshalled rather than compared as a map, because that is the form
			// the value reaches apply in, and a null member and an absent one are
			// both nil once decoded.
			got, err := json.Marshal(cp.Spec.ChartValues.Data)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(got))
		})
	}
}
