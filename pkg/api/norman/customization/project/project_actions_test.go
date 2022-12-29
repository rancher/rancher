package project

import (
	"testing"

	"github.com/stretchr/testify/assert"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
)

func TestParseKubeAPIServerArgs(t *testing.T) {
	testCases := []struct {
		name        string
		populateMap func(m rkev1.GenericMap)
		expected    map[string]string
	}{
		{
			name:        "no value",
			populateMap: func(m rkev1.GenericMap) {},
			expected:    make(map[string]string),
		},
		{
			name: "nil value",
			populateMap: func(m rkev1.GenericMap) {
				m.Data["kube-apiserver-arg"] = nil
			},
			expected: make(map[string]string),
		},
		{
			name: "invalid value type",
			populateMap: func(m rkev1.GenericMap) {
				m.Data["kube-apiserver-arg"] = []int{3, 2, 1}
			},
			expected: make(map[string]string),
		},
		{
			name: "invalid value type within list",
			populateMap: func(m rkev1.GenericMap) {
				m.Data["kube-apiserver-arg"] = []any{3, 2, 1}
			},
			expected: make(map[string]string),
		},
		{
			name: "invalid value within list",
			populateMap: func(m rkev1.GenericMap) {
				m.Data["kube-apiserver-arg"] = []any{"hey", "hello=world"}
			},
			expected: map[string]string{
				"hello": "world",
			},
		},
		{
			name: "valid list",
			populateMap: func(m rkev1.GenericMap) {
				m.Data["kube-apiserver-arg"] = []any{"hey=planet", "hello=world"}
			},
			expected: map[string]string{
				"hello": "world",
				"hey":   "planet",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cluster := &provisioningv1.Cluster{
				Spec: provisioningv1.ClusterSpec{
					RKEConfig: &provisioningv1.RKEConfig{
						RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
							MachineGlobalConfig: rkev1.GenericMap{
								Data: make(map[string]any),
							},
						},
					},
				},
			}
			tc.populateMap(cluster.Spec.RKEConfig.MachineGlobalConfig)
			res := parseKubeAPIServerArgs(cluster)
			assert.Equal(t, tc.expected, res)
		})
	}

}
