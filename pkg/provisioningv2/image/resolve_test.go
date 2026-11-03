package image

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResolveWithControlPlane(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		cp       *rkev1.RKEControlPlane
		global   string
		expected string
	}{
		{
			name:  "nil control plane falls back to global registry",
			image: "rancher/rancher-agent:v2.15.0",
			cp:    nil,
			// global is set via withGlobalRegistry below
			global:   "global.registry.io",
			expected: "global.registry.io/rancher/rancher-agent:v2.15.0",
		},
		{
			name:  "control plane with machineGlobalConfig registry",
			image: "rancher/rancher-agent:v2.15.0",
			cp: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						MachineGlobalConfig: rkev1.GenericMap{
							Data: map[string]any{
								"system-default-registry": "cp.registry.io",
							},
						},
					},
				},
			},
			expected: "cp.registry.io/rancher/rancher-agent:v2.15.0",
		},
		{
			name:  "control plane with machineSelectorConfig registry",
			image: "rancher/rancher-agent:v2.15.0",
			cp: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						MachineSelectorConfig: []rkev1.RKESystemConfig{
							{
								MachineLabelSelector: nil,
								Config: rkev1.GenericMap{
									Data: map[string]any{
										"system-default-registry": "selector.registry.io",
									},
								},
							},
						},
					},
				},
			},
			expected: "selector.registry.io/rancher/rancher-agent:v2.15.0",
		},
		{
			name:  "control plane with no registry and no global falls back to unmodified image",
			image: "rancher/rancher-agent:v2.15.0",
			cp: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{},
			},
			global:   "",
			expected: "rancher/rancher-agent:v2.15.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withGlobalRegistry(t, tt.global)
			got := ResolveWithControlPlane(tt.image, tt.cp)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestResolveWithCluster(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		cluster  *v1.Cluster
		global   string
		expected string
	}{
		{
			name:     "nil cluster falls back to global registry",
			image:    "rancher/rancher-agent:v2.15.0",
			cluster:  nil,
			global:   "global.registry.io",
			expected: "global.registry.io/rancher/rancher-agent:v2.15.0",
		},
		{
			name:  "cluster with machineGlobalConfig registry",
			image: "rancher/rancher-agent:v2.15.0",
			cluster: &v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							MachineGlobalConfig: rkev1.GenericMap{
								Data: map[string]any{
									"system-default-registry": "cluster.registry.io",
								},
							},
						},
					},
				},
			},
			expected: "cluster.registry.io/rancher/rancher-agent:v2.15.0",
		},
		{
			name:  "nil RKEConfig falls back to global registry",
			image: "rancher/rancher-agent:v2.15.0",
			cluster: &v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: nil,
				},
			},
			global:   "global.registry.io",
			expected: "global.registry.io/rancher/rancher-agent:v2.15.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withGlobalRegistry(t, tt.global)
			got := ResolveWithCluster(tt.image, tt.cluster)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetPrivateRepoURLFromCluster(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *v1.Cluster
		global      string
		expectedURL string
	}{
		{
			name:        "nil cluster returns global registry",
			cluster:     nil,
			global:      "global.registry.io",
			expectedURL: "global.registry.io",
		},
		{
			name: "cluster with nil RKEConfig returns global registry",
			cluster: &v1.Cluster{
				Spec: v1.ClusterSpec{RKEConfig: nil},
			},
			global:      "global.registry.io",
			expectedURL: "global.registry.io",
		},
		{
			name: "cluster with machineGlobalConfig registry returns cluster registry",
			cluster: &v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							MachineGlobalConfig: rkev1.GenericMap{
								Data: map[string]any{
									"system-default-registry": "cluster.registry.io",
								},
							},
						},
					},
				},
			},
			global:      "global.registry.io",
			expectedURL: "cluster.registry.io",
		},
		{
			name: "cluster with machineSelectorConfig (nil selector) returns that registry",
			cluster: &v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							MachineSelectorConfig: []rkev1.RKESystemConfig{
								{
									MachineLabelSelector: nil,
									Config: rkev1.GenericMap{
										Data: map[string]any{
											"system-default-registry": "selector.registry.io",
										},
									},
								},
							},
						},
					},
				},
			},
			global:      "global.registry.io",
			expectedURL: "selector.registry.io",
		},
		{
			name: "machineSelectorConfig with non-nil label selector is skipped, falls back to global",
			cluster: &v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							MachineSelectorConfig: []rkev1.RKESystemConfig{
								{
									MachineLabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{"role": "worker"},
									},
									Config: rkev1.GenericMap{
										Data: map[string]any{
											"system-default-registry": "worker.registry.io",
										},
									},
								},
							},
						},
					},
				},
			},
			global:      "global.registry.io",
			expectedURL: "global.registry.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withGlobalRegistry(t, tt.global)
			got, _ := GetPrivateRepoURLFromCluster(tt.cluster)
			assert.Equal(t, tt.expectedURL, got)
		})
	}
}

func TestGetPrivateRepoURLFromControlPlane(t *testing.T) {
	tests := []struct {
		name           string
		cp             *rkev1.RKEControlPlane
		global         string
		expectedURL    string
		expectedGlobal bool
	}{
		{
			name:           "nil control plane returns global registry with isGlobal=true",
			cp:             nil,
			global:         "global.registry.io",
			expectedURL:    "global.registry.io",
			expectedGlobal: true,
		},
		{
			name: "control plane with machineGlobalConfig registry returns it with isGlobal=false",
			cp: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						MachineGlobalConfig: rkev1.GenericMap{
							Data: map[string]any{
								"system-default-registry": "cp.registry.io",
							},
						},
					},
				},
			},
			global:         "global.registry.io",
			expectedURL:    "cp.registry.io",
			expectedGlobal: false,
		},
		{
			name: "control plane with no registry returns global with isGlobal=true",
			cp: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{},
			},
			global:         "global.registry.io",
			expectedURL:    "global.registry.io",
			expectedGlobal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withGlobalRegistry(t, tt.global)
			gotURL, gotGlobal := GetPrivateRepoURLFromControlPlane(tt.cp)
			assert.Equal(t, tt.expectedURL, gotURL)
			assert.Equal(t, tt.expectedGlobal, gotGlobal)
		})
	}
}

func TestGetPrivateRepoSecretFromCluster(t *testing.T) {
	const (
		clusterRegistry = "cluster.registry.io"
		globalRegistry  = "global.registry.io"
	)

	tests := []struct {
		name           string
		cluster        *v1.Cluster
		globalRegistry string
		globalSecrets  string
		expectedSecret string
	}{
		{
			name:           "nil cluster with no global pull secrets returns empty string",
			cluster:        nil,
			globalRegistry: globalRegistry,
			globalSecrets:  "",
			expectedSecret: "",
		},
		{
			name:           "nil cluster with global pull secrets returns first secret",
			cluster:        nil,
			globalRegistry: globalRegistry,
			globalSecrets:  "global-secret,other-secret",
			expectedSecret: "global-secret",
		},
		{
			name:           "nil cluster with global pull secrets trims whitespace",
			cluster:        nil,
			globalRegistry: globalRegistry,
			globalSecrets:  "  trimmed-secret  , other-secret",
			expectedSecret: "trimmed-secret",
		},
		{
			name: "cluster SDR explicitly set to same hostname as global SDR, does not use global pull secrets",
			cluster: &v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							MachineGlobalConfig: rkev1.GenericMap{
								Data: map[string]any{
									"system-default-registry": globalRegistry,
								},
							},
						},
					},
				},
			},
			globalRegistry: globalRegistry,
			globalSecrets:  "global-secret",
			expectedSecret: "",
		},
		{
			name: "cluster with matching registry config returns its auth secret",
			cluster: &v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							MachineGlobalConfig: rkev1.GenericMap{
								Data: map[string]any{
									"system-default-registry": clusterRegistry,
								},
							},
							Registries: &rkev1.Registry{
								Configs: map[string]rkev1.RegistryConfig{
									clusterRegistry: {
										AuthConfigSecretName: "my-cluster-secret",
									},
								},
							},
						},
					},
				},
			},
			globalRegistry: globalRegistry,
			globalSecrets:  "global-secret",
			expectedSecret: "my-cluster-secret",
		},
		// The next two cases confirm that when a cluster defines its own SDR (different
		// from the global), global pull secrets are never used, even if the cluster has
		// no matching registry config entry of its own.
		{
			name: "cluster SDR differs from global SDR with unmatched registry config does not use global pull secrets",
			cluster: &v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							MachineGlobalConfig: rkev1.GenericMap{
								Data: map[string]any{
									"system-default-registry": clusterRegistry,
								},
							},
							Registries: &rkev1.Registry{
								Configs: map[string]rkev1.RegistryConfig{
									"other.registry.io": {
										AuthConfigSecretName: "other-secret",
									},
								},
							},
						},
					},
				},
			},
			globalRegistry: globalRegistry,
			globalSecrets:  "global-secret",
			expectedSecret: "",
		},
		{
			name: "cluster SDR differs from global SDR with nil registries does not use global pull secrets",
			cluster: &v1.Cluster{
				Spec: v1.ClusterSpec{
					RKEConfig: &v1.RKEConfig{
						ClusterConfiguration: rkev1.ClusterConfiguration{
							MachineGlobalConfig: rkev1.GenericMap{
								Data: map[string]any{
									"system-default-registry": clusterRegistry,
								},
							},
							Registries: nil,
						},
					},
				},
			},
			globalRegistry: globalRegistry,
			globalSecrets:  "global-secret",
			expectedSecret: "",
		},
		{
			name:           "global SDR is malformed, get first valid entry",
			cluster:        &v1.Cluster{},
			globalRegistry: globalRegistry,
			globalSecrets:  ", global-secret",
			expectedSecret: "global-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withGlobalRegistry(t, tt.globalRegistry)
			withGlobalPullSecrets(t, tt.globalSecrets)
			got, _ := GetPrivateRepoSecretFromCluster(tt.cluster)
			assert.Equal(t, tt.expectedSecret, got)
		})
	}
}

func TestGetDesiredAgentImage(t *testing.T) {
	const agentImageDefault = "rancher/rancher-agent:v2.15.0"

	tests := []struct {
		name        string
		cp          *rkev1.RKEControlPlane
		mgmtCluster *v3.Cluster
		agentImage  string
		global      string
		expected    string
	}{
		{
			name: "AgentImageOverride takes highest priority",
			cp:   nil,
			mgmtCluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						DesiredAgentImage:  "desired-image:v1",
						AgentImageOverride: "override-image:v1",
					},
				},
			},
			expected: "override-image:v1",
		},
		{
			name: "DesiredAgentImage is used when AgentImageOverride is empty",
			cp:   nil,
			mgmtCluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						DesiredAgentImage: "desired-image:v1",
					},
				},
			},
			expected: "desired-image:v1",
		},
		{
			name:       "empty DesiredAgentImage resolves from settings",
			cp:         nil,
			agentImage: agentImageDefault,
			global:     "",
			mgmtCluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
			expected: agentImageDefault,
		},
		{
			name:       "\"fixed\" DesiredAgentImage resolves from settings",
			cp:         nil,
			agentImage: agentImageDefault,
			global:     "",
			mgmtCluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						DesiredAgentImage: "fixed",
					},
				},
			},
			expected: agentImageDefault,
		},
		{
			name:       "empty DesiredAgentImage with global registry resolves and prefixes image",
			cp:         nil,
			agentImage: "rancher/rancher-agent:v2.15.0",
			global:     "my.registry.io",
			mgmtCluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
			expected: "my.registry.io/rancher/rancher-agent:v2.15.0",
		},
		{
			name:       "control plane registry takes precedence over global",
			agentImage: "rancher/rancher-agent:v2.15.0",
			global:     "global.registry.io",
			cp: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						MachineGlobalConfig: rkev1.GenericMap{
							Data: map[string]any{
								"system-default-registry": "cp.registry.io",
							},
						},
					},
				},
			},
			mgmtCluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{},
				},
			},
			expected: "cp.registry.io/rancher/rancher-agent:v2.15.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withGlobalRegistry(t, tt.global)
			if tt.agentImage != "" {
				withAgentImage(t, tt.agentImage)
			}
			got := GetDesiredAgentImage(tt.cp, tt.mgmtCluster)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetDesiredAuthImage(t *testing.T) {
	const authImageDefault = "rancher/kube-api-auth:v0.2.2"

	tests := []struct {
		name        string
		cp          *rkev1.RKEControlPlane
		mgmtCluster *v3.Cluster
		authImage   string
		global      string
		expected    string
	}{
		{
			name: "LocalClusterAuthEndpoint disabled returns empty string",
			cp:   nil,
			mgmtCluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						LocalClusterAuthEndpoint: v3.LocalClusterAuthEndpoint{
							Enabled: false,
						},
						DesiredAuthImage: "some-image:v1",
					},
				},
			},
			expected: "",
		},
		{
			name: "LocalClusterAuthEndpoint enabled returns DesiredAuthImage",
			cp:   nil,
			mgmtCluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						LocalClusterAuthEndpoint: v3.LocalClusterAuthEndpoint{
							Enabled: true,
						},
						DesiredAuthImage: "desired-auth-image:v1",
					},
				},
			},
			expected: "desired-auth-image:v1",
		},
		{
			name:      "LocalClusterAuthEndpoint enabled, empty DesiredAuthImage resolves from settings",
			cp:        nil,
			authImage: authImageDefault,
			global:    "",
			mgmtCluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						LocalClusterAuthEndpoint: v3.LocalClusterAuthEndpoint{
							Enabled: true,
						},
					},
				},
			},
			expected: authImageDefault,
		},
		{
			name:      "LocalClusterAuthEndpoint enabled, \"fixed\" DesiredAuthImage resolves from settings",
			cp:        nil,
			authImage: authImageDefault,
			global:    "",
			mgmtCluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						LocalClusterAuthEndpoint: v3.LocalClusterAuthEndpoint{
							Enabled: true,
						},
						DesiredAuthImage: "fixed",
					},
				},
			},
			expected: authImageDefault,
		},
		{
			name:      "LocalClusterAuthEndpoint enabled, global registry prefixes resolved image",
			cp:        nil,
			authImage: "rancher/kube-api-auth:v0.2.2",
			global:    "my.registry.io",
			mgmtCluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						LocalClusterAuthEndpoint: v3.LocalClusterAuthEndpoint{
							Enabled: true,
						},
					},
				},
			},
			expected: "my.registry.io/rancher/kube-api-auth:v0.2.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withGlobalRegistry(t, tt.global)
			if tt.authImage != "" {
				withAuthImage(t, tt.authImage)
			}
			got := GetDesiredAuthImage(tt.cp, tt.mgmtCluster)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// withGlobalRegistry sets the global SystemDefaultRegistry setting for the
// duration of the test and restores the original value on cleanup.
func withGlobalRegistry(t *testing.T, reg string) {
	t.Helper()
	orig := settings.SystemDefaultRegistry.Get()
	if err := settings.SystemDefaultRegistry.Set(reg); err != nil {
		t.Fatalf("failed to set SystemDefaultRegistry: %v", err)
	}
	t.Cleanup(func() {
		if err := settings.SystemDefaultRegistry.Set(orig); err != nil {
			t.Logf("failed to restore SystemDefaultRegistry: %v", err)
		}
	})
}

// withGlobalPullSecrets sets the global SystemDefaultRegistryPullSecrets
// setting for the duration of the test and restores the original value on cleanup.
func withGlobalPullSecrets(t *testing.T, secrets string) {
	t.Helper()
	orig := settings.SystemDefaultRegistryPullSecrets.Get()
	if err := settings.SystemDefaultRegistryPullSecrets.Set(secrets); err != nil {
		t.Fatalf("failed to set SystemDefaultRegistryPullSecrets: %v", err)
	}
	t.Cleanup(func() {
		if err := settings.SystemDefaultRegistryPullSecrets.Set(orig); err != nil {
			t.Logf("failed to restore SystemDefaultRegistryPullSecrets: %v", err)
		}
	})
}

func withAgentImage(t *testing.T, img string) {
	t.Helper()
	orig := settings.AgentImage.Get()
	if err := settings.AgentImage.Set(img); err != nil {
		t.Fatalf("failed to set AgentImage: %v", err)
	}
	t.Cleanup(func() {
		if err := settings.AgentImage.Set(orig); err != nil {
			t.Logf("failed to restore AgentImage: %v", err)
		}
	})
}

func withAuthImage(t *testing.T, img string) {
	t.Helper()
	orig := settings.AuthImage.Get()
	if err := settings.AuthImage.Set(img); err != nil {
		t.Fatalf("failed to set AuthImage: %v", err)
	}
	t.Cleanup(func() {
		if err := settings.AuthImage.Set(orig); err != nil {
			t.Logf("failed to restore AuthImage: %v", err)
		}
	})
}
