package provisioningcluster

import (
	"encoding/json"
	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
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

func chartValuesCluster(t *testing.T, chartValues map[string]any) *provv1.Cluster {
	t.Helper()
	return &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "fleet-default"},
		Spec: provv1.ClusterSpec{
			RKEConfig: &provv1.RKEConfig{
				ClusterConfiguration: rkev1.ClusterConfiguration{
					ChartValues: rkev1.GenericMap{Data: chartValues},
				},
			},
		},
	}
}

// TestRKEControlPlaneClusterSpecAnnotationChartValues asserts that the cluster
// spec annotation rkeControlPlane() writes carries chart values verbatim,
// including explicit nulls.
func TestRKEControlPlaneClusterSpecAnnotationChartValues(t *testing.T) {
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
			cp, err := rkeControlPlane(chartValuesCluster(t, tt.chartValues))
			require.NoError(t, err)

			spec, err := snapshotutil.DecompressClusterSpec(cp.Annotations[capr.ClusterSpecAnnotation])
			require.NoError(t, err)
			require.NotNil(t, spec.RKEConfig)

			got, err := json.Marshal(spec.RKEConfig.ChartValues.Data)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(got))
		})
	}
}
