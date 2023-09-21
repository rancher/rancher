package planner

import (
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/stretchr/testify/assert"
)

func TestIsCalico(t *testing.T) {
	tests := []struct {
		name         string
		controlPlane *rkev1.RKEControlPlane
		runtime      string
		expected     bool
	}{
		{
			name: "calico rke2",
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
						MachineGlobalConfig: rkev1.GenericMap{
							Data: map[string]any{
								"cni": "calico",
							},
						},
					},
				},
			},
			runtime:  capr.RuntimeRKE2,
			expected: true,
		},
		{
			name: "calico+multus rke2",
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
						MachineGlobalConfig: rkev1.GenericMap{
							Data: map[string]any{
								"cni": "calico+multus",
							},
						},
					},
				},
			},
			runtime:  capr.RuntimeRKE2,
			expected: true,
		},
		{
			name: "mispelled calico rke2",
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
						MachineGlobalConfig: rkev1.GenericMap{
							Data: map[string]any{
								"cni": "calicoo",
							},
						},
					},
				},
			},
			runtime:  capr.RuntimeRKE2,
			expected: false,
		},
		{
			name: "calico k3s",
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					RKEClusterSpecCommon: rkev1.RKEClusterSpecCommon{
						MachineGlobalConfig: rkev1.GenericMap{
							Data: map[string]any{
								"cni": "calico",
							},
						},
					},
				},
			},
			runtime:  capr.RuntimeK3S,
			expected: false,
		},
		{
			name:         "no cni rke2",
			controlPlane: &rkev1.RKEControlPlane{},
			runtime:      capr.RuntimeRKE2,
			expected:     true,
		},
		{
			name:         "no cni k3s",
			controlPlane: &rkev1.RKEControlPlane{},
			runtime:      capr.RuntimeRKE2,
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isCalico(tt.controlPlane, tt.runtime))
		})
	}
}

func TestReplaceCACertAndPortForProbes(t *testing.T) {
	tests := []struct {
		name        string
		probe       plan.Probe
		cacert      string
		address     string
		port        string
		expected    plan.Probe
		expectedErr error
	}{
		{
			name:        "empty cacert",
			probe:       plan.Probe{},
			cacert:      "",
			address:     "",
			port:        "",
			expected:    plan.Probe{},
			expectedErr: errEmptyCACert,
		},
		{
			name:        "empty port",
			probe:       plan.Probe{},
			cacert:      "test",
			address:     "rancher.com",
			port:        "",
			expected:    plan.Probe{},
			expectedErr: errEmptyPort,
		},
		{
			name:        "empty address",
			probe:       plan.Probe{},
			cacert:      "test",
			address:     "",
			port:        "1234",
			expected:    plan.Probe{},
			expectedErr: errEmptyAddress,
		},
		{
			name: "URL with specifier",
			probe: plan.Probe{
				HTTPGetAction: plan.HTTPGetAction{
					CACert: "test",
					URL:    "https://%s:%s",
				},
			},
			cacert:  "test",
			address: "rancher.com",
			port:    "1234",
			expected: plan.Probe{
				HTTPGetAction: plan.HTTPGetAction{
					CACert: "test",
					URL:    "https://rancher.com:1234",
				},
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if probe, err := replaceCACertAndPortForProbes(tt.probe, tt.cacert, tt.address, tt.port); err != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.Equal(t, tt.expected, probe)
			}
		})
	}
}

func TestRenderProbes(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]plan.Probe
		runtime  string
		expected map[string]plan.Probe
	}{
		{
			name:     "nil",
			input:    nil,
			runtime:  "",
			expected: map[string]plan.Probe{},
		},
		{
			name:     "no probes",
			input:    map[string]plan.Probe{},
			runtime:  "",
			expected: map[string]plan.Probe{},
		},
		{
			name: "simple probe",
			input: map[string]plan.Probe{
				"a": {
					HTTPGetAction: plan.HTTPGetAction{
						CACert:     "cacert",
						ClientCert: "clientcert",
						ClientKey:  "clientkey",
					},
				},
			},
			runtime: capr.RuntimeRKE2,
			expected: map[string]plan.Probe{
				"a": {
					HTTPGetAction: plan.HTTPGetAction{
						CACert:     "cacert",
						ClientCert: "clientcert",
						ClientKey:  "clientkey",
					},
				},
			},
		},
		{
			name: "replace probe",
			input: map[string]plan.Probe{
				"a": {
					HTTPGetAction: plan.HTTPGetAction{
						CACert:     "%s/cacert",
						ClientCert: "%s/clientcert",
						ClientKey:  "%s/clientkey",
					},
				},
			},
			runtime: capr.RuntimeRKE2,
			expected: map[string]plan.Probe{
				"a": {
					HTTPGetAction: plan.HTTPGetAction{
						CACert:     "rke2/cacert",
						ClientCert: "rke2/clientcert",
						ClientKey:  "rke2/clientkey",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, replaceRuntimeForProbes(tt.input, tt.runtime))
		})
	}
}

func TestReplaceURLForProbes(t *testing.T) {
	tests := []struct {
		name            string
		input           map[string]plan.Probe
		loopbackAddress string
		expected        map[string]plan.Probe
	}{
		{
			name:            "nil",
			input:           nil,
			loopbackAddress: "",
			expected:        map[string]plan.Probe{},
		},
		{
			name:            "no probes",
			input:           map[string]plan.Probe{},
			loopbackAddress: "",
			expected:        map[string]plan.Probe{},
		},
		{
			name: "simple probe",
			input: map[string]plan.Probe{
				"a": {
					HTTPGetAction: plan.HTTPGetAction{
						URL: "https://127.0.0.1:1234/test",
					},
				},
			},
			loopbackAddress: "",
			expected: map[string]plan.Probe{
				"a": {
					HTTPGetAction: plan.HTTPGetAction{
						URL: "https://127.0.0.1:1234/test",
					},
				},
			},
		},
		{
			name: "replace ipv4 probe",
			input: map[string]plan.Probe{
				"a": {
					HTTPGetAction: plan.HTTPGetAction{
						URL: "https://%s:1234/test",
					},
				},
			},
			loopbackAddress: "127.0.0.1",
			expected: map[string]plan.Probe{
				"a": {
					HTTPGetAction: plan.HTTPGetAction{
						URL: "https://127.0.0.1:1234/test",
					},
				},
			},
		},
		{
			name: "replace ipv6 probe",
			input: map[string]plan.Probe{
				"a": {
					HTTPGetAction: plan.HTTPGetAction{
						URL: "https://%s:1234/test",
					},
				},
			},
			loopbackAddress: "[::1]",
			expected: map[string]plan.Probe{
				"a": {
					HTTPGetAction: plan.HTTPGetAction{
						URL: "https://[::1]:1234/test",
					},
				},
			},
		},
		{
			name: "replace dual probe",
			input: map[string]plan.Probe{
				"a": {
					HTTPGetAction: plan.HTTPGetAction{
						URL: "https://%s:1234/test",
					},
				},
			},
			loopbackAddress: "localhost",
			expected: map[string]plan.Probe{
				"a": {
					HTTPGetAction: plan.HTTPGetAction{
						URL: "https://localhost:1234/test",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, replaceURLForProbes(tt.input, tt.loopbackAddress))
		})
	}
}

func TestReplaceIfFormatSpecifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		runtime  string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			runtime:  "",
			expected: "",
		},
		{
			name:     "empty string with runtime",
			input:    "",
			runtime:  capr.RuntimeRKE2,
			expected: "",
		},
		{
			name:     "no format specifier",
			input:    "test",
			runtime:  capr.RuntimeRKE2,
			expected: "test",
		},
		{
			name:     "format specifier rke2",
			input:    "test/%s",
			runtime:  capr.RuntimeRKE2,
			expected: "test/rke2",
		},
		{
			name:     "format specifier k3s",
			input:    "test/%s",
			runtime:  capr.RuntimeK3S,
			expected: "test/k3s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, replaceIfFormatSpecifier(tt.input, tt.runtime))
		})
	}
}
