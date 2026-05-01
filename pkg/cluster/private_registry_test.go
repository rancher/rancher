package cluster

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/fleet"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestGetPrivateImportedClusterLevelRegistry tests the cluster-level registry extraction
// for imported/hosted clusters. These tests have no global state dependency.
func TestGetPrivateImportedClusterLevelRegistry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cluster  *v3.Cluster
		expected *PrivateRegistry
	}{
		{
			name:     "nil cluster",
			cluster:  nil,
			expected: nil,
		},
		{
			name: "nil ImportedConfig",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{},
			},
			expected: nil,
		},
		{
			name: "ImportedConfig with empty URL",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ImportedConfig: &v3.ImportedConfig{
						PrivateRegistryURL: "",
					},
				},
			},
			expected: nil,
		},
		{
			name: "ImportedConfig with URL and no pull secrets",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ImportedConfig: &v3.ImportedConfig{
						PrivateRegistryURL: "registry.example.com",
					},
				},
			},
			expected: &PrivateRegistry{
				URL:         "registry.example.com",
				PullSecrets: nil,
			},
		},
		{
			name: "ImportedConfig with URL and pull secrets",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ImportedConfig: &v3.ImportedConfig{
						PrivateRegistryURL:         "registry.example.com",
						PrivateRegistryPullSecrets: []string{"secret-a", "secret-b"},
					},
				},
			},
			expected: &PrivateRegistry{
				URL: "registry.example.com",
				PullSecrets: []corev1.SecretReference{
					{Namespace: fleet.ClustersDefaultNamespace, Name: "secret-a"},
					{Namespace: fleet.ClustersDefaultNamespace, Name: "secret-b"},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GetPrivateImportedClusterLevelRegistry(tt.cluster)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestGeneratePrivateRegistryDockerConfig tests the top-level docker config generation function.
// This test modifies global settings so it can't be run in parallel.
func TestGeneratePrivateRegistryDockerConfig(t *testing.T) {
	const registryURL = "0123456789abcdef.dkr.ecr.us-east-1.amazonaws.com"

	// dockerConfigForCreds builds the expected base64-encoded .dockerconfigjson using the
	// same BuildDockerConfigJson function used by production code, to avoid any JSON
	// serialization format assumptions.
	dockerConfigForCreds := func(t *testing.T, username, password string) string {
		t.Helper()
		raw, err := BuildDockerConfigJson(registryURL, username, password)
		require.NoError(t, err)
		return base64.StdEncoding.EncodeToString(raw)
	}

	mockSecrets := map[string]*corev1.Secret{}
	secretLister := &corefakes.SecretListerMock{
		GetFunc: func(namespace string, name string) (*corev1.Secret, error) {
			id := fmt.Sprintf("%s:%s", namespace, name)
			secret, ok := mockSecrets[id]
			if !ok {
				return nil, apierror.NewNotFound(schema.GroupResource{}, id)
			}
			return secret.DeepCopy(), nil
		},
	}

	tests := []struct {
		name                           string
		cluster                        *v3.Cluster
		secrets                        []*corev1.Secret
		globalSystemDefaultRegistry    string
		globalSystemDefaultPullSecrets string
		expectedURL                    string
		expectedPullSecrets            func(t *testing.T) []AgentPullSecret
		expectedErrContains            string
	}{
		{
			name:    "nil cluster",
			cluster: nil,
		},
		{
			name:        "v2prov cluster with basic-auth secret",
			expectedURL: registryURL,
			expectedPullSecrets: func(t *testing.T) []AgentPullSecret {
				return []AgentPullSecret{{
					Name:             "cattle-private-registry",
					DockerConfigJSON: dockerConfigForCreds(t, "testuser", "password"),
				}}
			},
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterSecrets: v3.ClusterSecrets{
							PrivateRegistrySecret: "test-secret",
							PrivateRegistryURL:    registryURL,
						},
					},
					FleetWorkspaceName: "fleet-default",
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "test-secret"},
					Type:       corev1.SecretTypeBasicAuth,
					Data: map[string][]byte{
						"username": []byte("testuser"),
						"password": []byte("password"),
					},
				},
			},
		},
		{
			name:        "v2prov cluster with rke auth-config secret",
			expectedURL: registryURL,
			expectedPullSecrets: func(t *testing.T) []AgentPullSecret {
				return []AgentPullSecret{{
					Name:             "cattle-private-registry",
					DockerConfigJSON: dockerConfigForCreds(t, "testuser", "password"),
				}}
			},
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterSecrets: v3.ClusterSecrets{
							PrivateRegistrySecret: "test-secret",
							PrivateRegistryURL:    registryURL,
						},
					},
					FleetWorkspaceName: "fleet-default",
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "test-secret"},
					Type:       "rke.cattle.io/auth-config",
					Data: map[string][]byte{
						"auth": []byte("testuser:password"),
					},
				},
			},
		},
		{
			name:        "v2prov cluster with no registry secret — returns URL only",
			expectedURL: registryURL,
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterSecrets: v3.ClusterSecrets{
							PrivateRegistryURL: registryURL,
						},
					},
					FleetWorkspaceName: "fleet-default",
				},
			},
		},
		{
			name:                "v2prov cluster where secret lookup fails",
			expectedURL:         registryURL,
			expectedErrContains: "fleet-default:test-secret",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterSecrets: v3.ClusterSecrets{
							PrivateRegistrySecret: "test-secret",
							PrivateRegistryURL:    registryURL,
						},
					},
					FleetWorkspaceName: "fleet-default",
				},
			},
		},
		{
			name:        "imported cluster with ImportedConfig and pull secrets",
			expectedURL: registryURL,
			expectedPullSecrets: func(t *testing.T) []AgentPullSecret {
				return []AgentPullSecret{{
					Name:             "imported-secret",
					DockerConfigJSON: dockerConfigForCreds(t, "importuser", "importpass"),
				}}
			},
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-j4nd7"},
				Spec: v3.ClusterSpec{
					ImportedConfig: &v3.ImportedConfig{
						PrivateRegistryURL:         registryURL,
						PrivateRegistryPullSecrets: []string{"imported-secret"},
					},
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: fleet.ClustersDefaultNamespace, Name: "imported-secret"},
					Type:       corev1.SecretTypeBasicAuth,
					Data: map[string][]byte{
						"username": []byte("importuser"),
						"password": []byte("importpass"),
					},
				},
			},
		},
		{
			name:        "imported cluster with ImportedConfig and no pull secrets — returns URL only",
			expectedURL: registryURL,
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-j4nd7"},
				Spec: v3.ClusterSpec{
					ImportedConfig: &v3.ImportedConfig{
						PrivateRegistryURL: registryURL,
					},
				},
			},
		},
		{
			name:                "imported cluster where pull secret lookup fails",
			expectedErrContains: "failed to get pull secret imported-secret",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-j4nd7"},
				Spec: v3.ClusterSpec{
					ImportedConfig: &v3.ImportedConfig{
						PrivateRegistryURL:         registryURL,
						PrivateRegistryPullSecrets: []string{"imported-secret"},
					},
				},
			},
		},
		{
			name:        "cluster name not matching MgmtNameRegexp skips secret generation",
			expectedURL: registryURL,
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-m-3j5d1"},
				Spec: v3.ClusterSpec{
					ImportedConfig: &v3.ImportedConfig{
						PrivateRegistryURL:         registryURL,
						PrivateRegistryPullSecrets: []string{"some-secret"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSecrets = make(map[string]*corev1.Secret)
			for _, s := range tt.secrets {
				mockSecrets[fmt.Sprintf("%s:%s", s.Namespace, s.Name)] = s
			}

			require.NoError(t, settings.SystemDefaultRegistry.Set(tt.globalSystemDefaultRegistry))
			require.NoError(t, settings.SystemDefaultRegistryPullSecrets.Set(tt.globalSystemDefaultPullSecrets))
			t.Cleanup(func() {
				settings.SystemDefaultRegistry.Set("")
				settings.SystemDefaultRegistryPullSecrets.Set("")
			})

			var expectedPullSecrets []AgentPullSecret
			if tt.expectedPullSecrets != nil {
				expectedPullSecrets = tt.expectedPullSecrets(t)
			}

			url, pullSecrets, err := GeneratePrivateRegistryEncodedDockerConfig(tt.cluster, secretLister)
			assert.Equal(t, tt.expectedURL, url)
			assert.Equal(t, expectedPullSecrets, pullSecrets)
			if tt.expectedErrContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectedErrContains)
			}
		})
	}
}

// TestGetPrivateRegistry tests the registry resolution precedence (cluster > global default).
// These tests mutate global settings so they cannot run in parallel.
func TestGetPrivateRegistry(t *testing.T) {
	tests := []struct {
		name                  string
		cluster               *v3.Cluster
		globalRegistryURL     string
		globalRegistrySecrets []string
		expectedRegistry      *PrivateRegistry
		expectedIsDefault     bool
	}{
		{
			name:              "nil cluster, no GSDR",
			cluster:           nil,
			expectedRegistry:  nil,
			expectedIsDefault: true,
		},
		{
			name:              "nil cluster, GSDR set",
			cluster:           nil,
			globalRegistryURL: "global.registry.example.com",
			expectedRegistry: &PrivateRegistry{
				URL:         "global.registry.example.com",
				PullSecrets: nil,
			},
			expectedIsDefault: true,
		},
		{
			name:                  "nil cluster, GSDR set with single pull secret",
			cluster:               nil,
			globalRegistryURL:     "global.registry.example.com",
			globalRegistrySecrets: []string{"imported-secret"},
			expectedRegistry: &PrivateRegistry{
				URL: "global.registry.example.com",
				PullSecrets: []corev1.SecretReference{
					{
						Namespace: namespaces.System,
						Name:      "imported-secret",
					},
				},
			},
			expectedIsDefault: true,
		},
		{
			name:                  "nil cluster, GSDR set with multiple pull secrets",
			cluster:               nil,
			globalRegistryURL:     "global.registry.example.com",
			globalRegistrySecrets: []string{"imported-secret", "imported-secret2"},
			expectedRegistry: &PrivateRegistry{
				URL: "global.registry.example.com",
				PullSecrets: []corev1.SecretReference{
					{
						Namespace: namespaces.System,
						Name:      "imported-secret",
					},
					{
						Namespace: namespaces.System,
						Name:      "imported-secret2",
					},
				},
			},
			expectedIsDefault: true,
		},
		{
			name: "cluster with ImportedConfig overrides GSDR",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ImportedConfig: &v3.ImportedConfig{
						PrivateRegistryURL: "cluster.registry.example.com",
					},
				},
			},
			globalRegistryURL: "global.registry.example.com",
			expectedRegistry: &PrivateRegistry{
				URL:         "cluster.registry.example.com",
				PullSecrets: nil,
			},
			expectedIsDefault: false,
		},
		{
			name: "cluster with ImportedConfig and pull secrets overrides GSDR",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ImportedConfig: &v3.ImportedConfig{
						PrivateRegistryURL: "cluster.registry.example.com",
						PrivateRegistryPullSecrets: []string{
							"imported-secret",
							"imported-secret2",
						},
					},
				},
			},
			globalRegistryURL: "global.registry.example.com",
			expectedRegistry: &PrivateRegistry{
				URL: "cluster.registry.example.com",
				PullSecrets: []corev1.SecretReference{
					{
						Namespace: "fleet-default",
						Name:      "imported-secret",
					},
					{
						Namespace: "fleet-default",
						Name:      "imported-secret2",
					},
				},
			},
			expectedIsDefault: false,
		},
		{
			name: "cluster without ImportedConfig falls back to GSDR",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{},
			},
			globalRegistryURL: "global.registry.example.com",
			expectedRegistry: &PrivateRegistry{
				URL:         "global.registry.example.com",
				PullSecrets: nil,
			},
			expectedIsDefault: true,
		},
		{
			name: "cluster without ImportedConfig falls back to GSDR with pull secrets",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{},
			},
			globalRegistryURL: "global.registry.example.com",
			globalRegistrySecrets: []string{
				"imported-secret",
				"imported-secret2",
			},
			expectedRegistry: &PrivateRegistry{
				URL: "global.registry.example.com",
				PullSecrets: []corev1.SecretReference{
					{
						Namespace: namespaces.System,
						Name:      "imported-secret",
					},
					{
						Namespace: namespaces.System,
						Name:      "imported-secret2",
					},
				},
			},
			expectedIsDefault: true,
		},
		{
			name: "cluster without ImportedConfig and no GSDR",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{},
			},
			expectedRegistry:  nil,
			expectedIsDefault: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, settings.SystemDefaultRegistry.Set(tt.globalRegistryURL))
			require.NoError(t, settings.SystemDefaultRegistryPullSecrets.Set(strings.Join(tt.globalRegistrySecrets, ",")))
			t.Cleanup(func() {
				settings.SystemDefaultRegistry.Set("")
				settings.SystemDefaultRegistryPullSecrets.Set("")
			})

			registry, isDefault := GetPrivateRegistry(tt.cluster)
			assert.Equal(t, tt.expectedRegistry, registry)
			assert.Equal(t, tt.expectedIsDefault, isDefault)
		})
	}
}

// TestGetPrivateRegistryURL tests the URL-only convenience wrapper.
// These tests mutate global settings so they cannot run in parallel.
func TestGetPrivateRegistryURL(t *testing.T) {
	tests := []struct {
		name              string
		cluster           *v3.Cluster
		globalRegistryURL string
		expectedURL       string
	}{
		{
			name:        "nil cluster, no registry",
			cluster:     nil,
			expectedURL: "",
		},
		{
			name:              "nil cluster, GSDR set",
			cluster:           nil,
			globalRegistryURL: "global.registry.example.com",
			expectedURL:       "global.registry.example.com",
		},
		{
			name:              "cluster with ImportedConfig",
			globalRegistryURL: "global.registry.example.com",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ImportedConfig: &v3.ImportedConfig{
						PrivateRegistryURL: "cluster.registry.example.com",
					},
				},
			},
			expectedURL: "cluster.registry.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, settings.SystemDefaultRegistry.Set(tt.globalRegistryURL))
			t.Cleanup(func() { settings.SystemDefaultRegistry.Set("") })

			assert.Equal(t, tt.expectedURL, GetPrivateRegistryURL(tt.cluster))
		})
	}
}

// TestPrivateRegistryPullSecretNamesAsSlice tests the name extraction helper.
func TestPrivateRegistryPullSecretNamesAsSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		registry PrivateRegistry
		expected []string
	}{
		{
			name:     "empty pull secrets",
			registry: PrivateRegistry{},
			expected: nil,
		},
		{
			name: "single pull secret",
			registry: PrivateRegistry{
				PullSecrets: []corev1.SecretReference{
					{Namespace: "cattle-system", Name: "my-secret"},
				},
			},
			expected: []string{"my-secret"},
		},
		{
			name: "multiple pull secrets",
			registry: PrivateRegistry{
				PullSecrets: []corev1.SecretReference{
					{Namespace: "cattle-system", Name: "secret-a"},
					{Namespace: "fleet-default", Name: "secret-b"},
				},
			},
			expected: []string{"secret-a", "secret-b"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.registry.PullSecretNamesAsSlice())
		})
	}
}

// TestPrivateRegistryPullSecretsAsObjectReferences tests the LocalObjectReference conversion helper.
func TestPrivateRegistryPullSecretsAsObjectReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		registry PrivateRegistry
		expected []corev1.LocalObjectReference
	}{
		{
			name:     "empty pull secrets",
			registry: PrivateRegistry{},
			expected: nil,
		},
		{
			name: "single pull secret",
			registry: PrivateRegistry{
				PullSecrets: []corev1.SecretReference{
					{Namespace: "cattle-system", Name: "my-secret"},
				},
			},
			expected: []corev1.LocalObjectReference{
				{Name: "my-secret"},
			},
		},
		{
			name: "multiple pull secrets — namespace is dropped",
			registry: PrivateRegistry{
				PullSecrets: []corev1.SecretReference{
					{Namespace: "cattle-system", Name: "secret-a"},
					{Namespace: "fleet-default", Name: "secret-b"},
				},
			},
			expected: []corev1.LocalObjectReference{
				{Name: "secret-a"},
				{Name: "secret-b"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.registry.PullSecretsAsObjectReferences())
		})
	}
}
