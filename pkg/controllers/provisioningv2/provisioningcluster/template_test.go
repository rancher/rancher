package provisioningcluster

import (
	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/stretchr/testify/assert"
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
