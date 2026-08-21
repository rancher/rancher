package provisioningcluster

import (
	"encoding/json"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
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

func TestRKEControlPlaneChartValuesJSON(t *testing.T) {
	tests := []struct {
		name        string
		chartValues map[string]any
		expected    string
	}{
		{
			name:        "nil chart values produce an empty string",
			chartValues: nil,
			expected:    "",
		},
		{
			name:        "empty chart values produce an empty string",
			chartValues: map[string]any{},
			expected:    "",
		},
		{
			name: "chart values are serialized",
			chartValues: map[string]any{
				"rke2-coredns": map[string]any{"replicas": 2},
			},
			expected: `{"rke2-coredns":{"replicas":2}}`,
		},
		{
			// carry an explicit null across the merge patch to the RKEControlPlane.
			name: "an explicit null is serialized",
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
			assert.Equal(t, tt.expected, cp.Spec.ChartValuesJSON)
		})
	}
}

// Wrangler's apply updates an existing RKEControlPlane with a
// types.MergePatchType patch built by jsonmergepatch.CreateThreeWayJSONMergePatch,
// because rke.cattle.io/v1 is a CRD and so is not registered in the client-go scheme.
func TestRKEControlPlaneChartValuesSurviveMergePatch(t *testing.T) {
	// The cluster as it was when the RKEControlPlane was last applied.
	before, err := rkeControlPlane(chartValuesCluster(t, map[string]any{
		"rke2-coredns": map[string]any{
			"resources": map[string]any{
				"limits": map[string]any{"memory": "128Mi"},
			},
		},
	}))
	require.NoError(t, err)

	// The user now sets cpu to an explicit null to drop the chart default.
	after, err := rkeControlPlane(chartValuesCluster(t, map[string]any{
		"rke2-coredns": map[string]any{
			"resources": map[string]any{
				"limits": map[string]any{"cpu": nil, "memory": "130Mi"},
			},
		},
	}))
	require.NoError(t, err)

	original, err := json.Marshal(before)
	require.NoError(t, err)
	modified, err := json.Marshal(after)
	require.NoError(t, err)

	// current is the live object, which here matches what was last applied.
	patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(original, modified, original)
	require.NoError(t, err)

	patched, err := jsonpatch.MergePatch(original, patch)
	require.NoError(t, err)

	result := &rkev1.RKEControlPlane{}
	require.NoError(t, json.Unmarshal(patched, result))

	assert.JSONEq(t,
		`{"rke2-coredns":{"resources":{"limits":{"cpu":null,"memory":"130Mi"}}}}`,
		result.Spec.ChartValuesJSON,
		"the explicit null must survive the merge patch in the serialized field")

	// Contrast: the structured field loses the null, which is the bug this works
	// around. Asserting it keeps the workaround honest -- if a future change makes
	// the structured field null-safe, this fails and the serialized field can go.
	limits := result.Spec.ChartValues.Data["rke2-coredns"].(map[string]any)["resources"].(map[string]any)["limits"].(map[string]any)
	assert.NotContains(t, limits, "cpu",
		"the structured field is still expected to drop the null across a merge patch")
	assert.Equal(t, "130Mi", limits["memory"])
}
