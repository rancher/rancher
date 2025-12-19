package image

import (
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestResolve validates the resolve function correctly prepends registry URLs to image names
// and handles special cases like Dockerhub library images. Tests are run in parallel for efficiency.
func TestResolve(t *testing.T) {
	tests := []struct {
		name     string
		reg      string
		image    string
		expected string
	}{
		{
			name:     "empty registry returns original image",
			reg:      "",
			image:    "nginx:latest",
			expected: "nginx:latest",
		},
		{
			name:     "image already has registry prefix",
			reg:      "registry.example.com",
			image:    "registry.example.com/myapp:v1.0",
			expected: "registry.example.com/myapp:v1.0",
		},
		{
			name:     "dockerhub library image gets rancher prefix",
			reg:      "registry.example.com",
			image:    "nginx",
			expected: "registry.example.com/rancher/nginx",
		},
		{
			name:     "image with namespace gets registry prefix",
			reg:      "registry.example.com",
			image:    "myorg/myapp:v1.0",
			expected: "registry.example.com/myorg/myapp:v1.0",
		},
		{
			name:     "image with full path and tag",
			reg:      "private.registry.io",
			image:    "namespace/app:latest",
			expected: "private.registry.io/namespace/app:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := resolve(tt.reg, tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetPrivateRepoURL validates extraction of system-default-registry from machine configurations.
// It tests that the function correctly prioritizes machineGlobalConfig over machineSelectorConfig
// and falls back to the global system-default-registry setting. Tests run in parallel.
func TestGetPrivateRepoURL(t *testing.T) {
	// Store original setting and restore after test
	originalSetting := settings.SystemDefaultRegistry.Get()
	defer func() {
		_ = settings.SystemDefaultRegistry.Set(originalSetting)
	}()
	_ = settings.SystemDefaultRegistry.Set("global.registry.io")

	tests := []struct {
		name                   string
		machineGlobalConfig    rkev1.GenericMap
		machineSelectorConfig  []rkev1.RKESystemConfig
		expected               string
	}{
		{
			name: "registry from machineGlobalConfig",
			machineGlobalConfig: rkev1.GenericMap{
				Data: map[string]interface{}{
					"system-default-registry": "cluster.registry.io",
				},
			},
			machineSelectorConfig: nil,
			expected:              "cluster.registry.io",
		},
		{
			name: "registry from machineSelectorConfig without label selector",
			machineGlobalConfig: rkev1.GenericMap{
				Data: map[string]interface{}{},
			},
			machineSelectorConfig: []rkev1.RKESystemConfig{
				{
					MachineLabelSelector: nil,
					Config: rkev1.GenericMap{
						Data: map[string]interface{}{
							"system-default-registry": "selector.registry.io",
						},
					},
				},
			},
			expected: "selector.registry.io",
		},
		{
			name: "skip machineSelectorConfig with label selector",
			machineGlobalConfig: rkev1.GenericMap{
				Data: map[string]interface{}{},
			},
			machineSelectorConfig: []rkev1.RKESystemConfig{
				{
					MachineLabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"role": "worker"},
					},
					Config: rkev1.GenericMap{
						Data: map[string]interface{}{
							"system-default-registry": "labeled.registry.io",
						},
					},
				},
			},
			expected: "global.registry.io",
		},
		{
			name: "fallback to global setting",
			machineGlobalConfig: rkev1.GenericMap{
				Data: map[string]interface{}{},
			},
			machineSelectorConfig: nil,
			expected:              "global.registry.io",
		},
		{
			name: "machineGlobalConfig takes priority over machineSelectorConfig",
			machineGlobalConfig: rkev1.GenericMap{
				Data: map[string]interface{}{
					"system-default-registry": "global-config.registry.io",
				},
			},
			machineSelectorConfig: []rkev1.RKESystemConfig{
				{
					MachineLabelSelector: nil,
					Config: rkev1.GenericMap{
						Data: map[string]interface{}{
							"system-default-registry": "selector-config.registry.io",
						},
					},
				},
			},
			expected: "global-config.registry.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := getPrivateRepoURL(tt.machineGlobalConfig, tt.machineSelectorConfig)
			assert.Equal(t, tt.expected, result)
		})
	}
}
