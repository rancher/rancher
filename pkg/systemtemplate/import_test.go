package systemtemplate

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var update = flag.Bool("update", false, "update snapshot files with current test outputs")

func TestSystemTemplate_systemtemplate(t *testing.T) {
	mockSecrets := map[string]*corev1.Secret{}
	secretLister := &corefakes.SecretListerMock{
		GetFunc: func(namespace string, name string) (*corev1.Secret, error) {
			id := fmt.Sprintf("%s:%s", namespace, name)
			secret, ok := mockSecrets[fmt.Sprintf("%s:%s", namespace, name)]
			if !ok {
				return nil, apierror.NewNotFound(schema.GroupResource{}, id)
			}
			return secret.DeepCopy(), nil
		},
	}

	preemption := corev1.PreemptionPolicy("Never")

	tests := []struct {
		name           string
		cluster        *apimgmtv3.Cluster
		pcExists       bool
		agentImage     string
		authImage      string
		assetsImage    string
		namespace      string
		token          string
		url            string
		isPreBootstrap bool
		features       map[string]bool
		taints         []corev1.Taint
		mutator        namespace.Mutator
		secrets        map[string]*corev1.Secret

		expectedError string
	}{
		{
			name: "test-provisioned-import",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-prov",
				},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName:    "testing-rke2",
					ImportedConfig: &apimgmtv3.ImportedConfig{},
				},
				Status: apimgmtv3.ClusterStatus{
					Driver:   "imported",
					Provider: "rke2",
				},
			},
			taints: []corev1.Taint{
				{
					Key:       "key1",
					Value:     "value1",
					Effect:    corev1.TaintEffectNoSchedule,
					TimeAdded: &metav1.Time{}, // this should be stripped from tolerations
				},
				{
					Key:    "key2",
					Effect: corev1.TaintEffectPreferNoSchedule,
				},
			},
		},
		{
			name:     "test-provisioned-import with scheduling customization, initial registration",
			pcExists: false,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-prov",
				},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName:    "testing-rke2",
					ImportedConfig: &apimgmtv3.ImportedConfig{},
					ClusterSpecBase: apimgmtv3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &apimgmtv3.AgentDeploymentCustomization{
							SchedulingCustomization: &apimgmtv3.AgentSchedulingCustomization{
								PriorityClass: &apimgmtv3.PriorityClassSpec{
									Value:            123456,
									PreemptionPolicy: &preemption,
								},
								PodDisruptionBudget: &apimgmtv3.PodDisruptionBudgetSpec{
									MinAvailable: "1",
								},
							},
						},
					},
				},
				Status: apimgmtv3.ClusterStatus{
					Driver:   "imported",
					Provider: "rke2",
				},
			},
		},
		{
			name:     "test-provisioned-import with scheduling customization, cluster deploy creation",
			pcExists: true,
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-prov",
				},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName:    "testing-rke2",
					ImportedConfig: &apimgmtv3.ImportedConfig{},
					ClusterSpecBase: apimgmtv3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &apimgmtv3.AgentDeploymentCustomization{
							SchedulingCustomization: &apimgmtv3.AgentSchedulingCustomization{
								PriorityClass: &apimgmtv3.PriorityClassSpec{
									Value:            123456,
									PreemptionPolicy: &preemption,
								},
								PodDisruptionBudget: &apimgmtv3.PodDisruptionBudgetSpec{
									MinAvailable: "1",
								},
							},
						},
					},
				},
				Status: apimgmtv3.ClusterStatus{
					Driver:   "imported",
					Provider: "rke2",
				},
			},
		},
		{
			name: "test-provisioned-import-custom-agent",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-prov",
				},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName: "testing-rke2",
					ImportedConfig: &apimgmtv3.ImportedConfig{
						PrivateRegistryURL: "localhost:5001",
					},
				},
				Status: apimgmtv3.ClusterStatus{
					Driver:   "imported",
					Provider: "rke2",
				},
			},
			url:         "some-dummy-url",
			token:       "some-dummy-token",
			agentImage:  "my/agent:image",
			assetsImage: "rancher/assets:charts",
		},
		{
			name: "test-rancher-namespace-options-enabled",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-options",
				},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName:    "testing-namesapce-opotions",
					ImportedConfig: &apimgmtv3.ImportedConfig{},
				},
				Status: apimgmtv3.ClusterStatus{
					Driver:   "imported",
					Provider: "rke2",
				},
			},
			mutator: namespace.Mutator{
				Enabled: true,
				Annotations: map[string]string{
					"foo": "bar",
				},
				Labels: map[string]string{
					"baz": "quz",
				},
			},
			agentImage:  "my/agent:image",
			assetsImage: "rancher/assets:charts",
		},
		{
			name: "test-rancher-namespace-options-enabled-no-labels",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-options",
				},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName:    "testing-namesapce-opotions",
					ImportedConfig: &apimgmtv3.ImportedConfig{},
				},
				Status: apimgmtv3.ClusterStatus{
					Driver:   "imported",
					Provider: "rke2",
				},
			},
			mutator: namespace.Mutator{
				Enabled: true,
				Annotations: map[string]string{
					"foo": "bar",
				},
				Labels: map[string]string{},
			},
			agentImage:  "my/agent:image",
			assetsImage: "rancher/assets:charts",
		},
		{
			name: "test-rancher-namespace-options-enabled-no-annotations",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-namespace-options",
				},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName:    "testing-namesapce-opotions",
					ImportedConfig: &apimgmtv3.ImportedConfig{},
				},
				Status: apimgmtv3.ClusterStatus{
					Driver:   "imported",
					Provider: "rke2",
				},
			},
			mutator: namespace.Mutator{
				Enabled:     true,
				Annotations: map[string]string{},
				Labels: map[string]string{
					"baz": "quz",
				},
			},
			agentImage:  "my/agent:image",
			assetsImage: "rancher/assets:charts",
		},
		{
			name: "imported cluster with pull secrets renders imagePullSecrets and secret resources",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-abc12", // matches MgmtNameRegexp
				},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName: "test-imported-pull-secrets",
					ImportedConfig: &apimgmtv3.ImportedConfig{
						PrivateRegistryURL:         "my-registry.example.com",
						PrivateRegistryPullSecrets: []string{"my-pull-secret"},
					},
				},
				Status: apimgmtv3.ClusterStatus{
					Driver:   "imported",
					Provider: "rke2",
				},
			},
			agentImage:  "rancher/rancher-agent:v2.8.0",
			assetsImage: "rancher/assets:charts",
			token:       "test-token",
			url:         "https://rancher.example.com",
			secrets: map[string]*corev1.Secret{
				"fleet-default:my-pull-secret": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "fleet-default",
						Name:      "my-pull-secret",
					},
					Type: corev1.SecretTypeBasicAuth,
					Data: map[string][]byte{
						"username": []byte("testuser"),
						"password": []byte("testpass"),
					},
				},
			},
		},
		{
			name: "provisioned cluster name does not get system default pull secrets env var",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-m-abc12", // does NOT match MgmtNameRegexp
				},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName: "test-prov-no-system-secrets",
					ImportedConfig: &apimgmtv3.ImportedConfig{
						PrivateRegistryURL:         "my-registry.example.com",
						PrivateRegistryPullSecrets: []string{"my-pull-secret-rancher-managed-pull-secret"},
					},
				},
			},
			agentImage:  "rancher-agent:v2.8.0",
			assetsImage: "rancher/assets:charts",
			token:       "test-token",
			url:         "https://rancher.example.com",
			secrets: map[string]*corev1.Secret{
				"fleet-default:my-pull-secret-rancher-managed-pull-secret": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "fleet-default",
						Name:      "my-pull-secret-rancher-managed-pull-secret",
					},
					Type: corev1.SecretTypeBasicAuth,
					Data: map[string][]byte{
						"username": []byte("testuser"),
						"password": []byte("testpass"),
					},
				},
			},
		},
		{
			name: "imported cluster with multiple pull secrets",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-xyz99", // matches MgmtNameRegexp
				},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName: "test-multi-secrets",
					ImportedConfig: &apimgmtv3.ImportedConfig{
						PrivateRegistryURL:         "my-registry.example.com",
						PrivateRegistryPullSecrets: []string{"secret-one", "secret-two"},
					},
				},
				Status: apimgmtv3.ClusterStatus{
					Driver:   "imported",
					Provider: "rke2",
				},
			},
			agentImage:  "rancher-agent:v2.8.0",
			assetsImage: "rancher/assets:charts",
			token:       "test-token",
			url:         "https://rancher.example.com",
			secrets: map[string]*corev1.Secret{
				"fleet-default:secret-one": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "fleet-default",
						Name:      "secret-one",
					},
					Type: corev1.SecretTypeBasicAuth,
					Data: map[string][]byte{
						"username": []byte("user1"),
						"password": []byte("pass1"),
					},
				},
				"fleet-default:secret-two": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "fleet-default",
						Name:      "secret-two",
					},
					Type: corev1.SecretTypeBasicAuth,
					Data: map[string][]byte{
						"username": []byte("user2"),
						"password": []byte("pass2"),
					},
				},
			},
		},
		{
			name: "pull secret lookup failure returns error",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-abc12",
				},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName: "test-secret-failure",
					ImportedConfig: &apimgmtv3.ImportedConfig{
						PrivateRegistryURL:         "my-registry.example.com",
						PrivateRegistryPullSecrets: []string{"nonexistent-secret"},
					},
				},
				Status: apimgmtv3.ClusterStatus{
					Driver:   "imported",
					Provider: "rke2",
				},
			},
			agentImage:    "my-registry.example.com/rancher/rancher-agent:v2.8.0",
			assetsImage:   "rancher/assets:charts",
			token:         "test-token",
			url:           "https://rancher.example.com",
			expectedError: "\"fleet-default:nonexistent-secret\" not found",
		},
		{
			name: "pre-bootstrap renders bootstrap deployment with hostNetwork",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-preboot"},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName:    "test-preboot",
					ImportedConfig: &apimgmtv3.ImportedConfig{},
				},
				Status: apimgmtv3.ClusterStatus{
					Driver:   "imported",
					Provider: "rke2",
				},
			},
			agentImage:     "rancher/rancher-agent:v2.8.0",
			assetsImage:    "rancher/assets:charts",
			token:          "test-token",
			url:            "https://rancher.example.com",
			isPreBootstrap: true,
		},
		{
			name: "test-kube-api-auth-enabled",

			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-auth",
				},
				Spec: apimgmtv3.ClusterSpec{
					DisplayName:    "testing-kube-api-auth",
					ImportedConfig: &apimgmtv3.ImportedConfig{},
				},
				Status: apimgmtv3.ClusterStatus{
					Driver:   "imported",
					Provider: "rke2",
				},
			},

			agentImage: "my/agent:image",
			authImage:  "my/kube-api-auth:image",
			url:        "https://example.com",
			token:      "dummy-token",

			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "e99508716bf42e1d9190e6957b5c7745d62d94a0e6b9f3d7ada4f656afcb6efe",
			},

			expectedDaemonSetHashes: map[string]string{
				"kube-api-auth": "71cdcb54a60bab2f82a2f65c97d3ef2a133f1780256f6de6c34a50e9741e63fe",
			},

			expectedClusterRoleHashes: map[string]string{
				"proxy-clusterrole-kubeapiserver": "0b1d7f692252b3f498855fa24f669499ba1c061d0ae0eab0db2bb570bc25e63c",
				"cattle-admin":                    "d2b6b43774ce046f3e4e157b94167d6be596d697c3c9411d4ef4d6f29c2d5fde",
				"kube-api-auth":                   "5edba6ae199bce61bbbe1c8c689a6900981e3320e1c3b16b08cba8be1ea1b11b",
			},

			expectedClusterRoleBindingHashes: map[string]string{
				"proxy-role-binding-kubernetes-master": "8e33b2e67243b5a87012489fcd12b4e805c6b6b3c3c2bb4063eee04ca7bc372e",
				"cattle-admin-binding":                 "d646e3b685d8f931a11f4938e4c95a97151286fa391ef03898e6d44f6827cf16",
				"kube-api-auth":                        "50d6e64be34295d7631e5e25323146e8a9f009a992acc924a1109e4965c67193",
			},

			expectedNamespaceHashes: map[string]string{
				"cattle-system": "53b1582048d8703999612a3b41f7301b4136e8dd3041d57e9a59c97e76dfa564",
			},

			expectedServiceHashes: map[string]string{
				"cattle-cluster-agent": "03b629bf7287d1a70f31fdf138ea5ec38201040e757b21a808ea0d413e27d65f",
			},

			expectedServiceAccountHashes: map[string]string{
				"cattle":        "ba41ec07896a1e2d2319c0ca1405c81faf4ad4c7c0a3c183909860531863202b",
				"kube-api-auth": "0d766aa7dcaa099ce355d8baaab533beb33b7766e54fa74fac8f9393c4ed18de",
			},

			expectedSecretHashes: map[string]string{
				"cattle-credentials-8f25b52916": "24570c6bceef80892243253fefb8ac4d8651e23808633d7b532ca04f8472caa8",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mockSecrets = tt.secrets
			var b bytes.Buffer
			if tt.cluster.Spec.ImportedConfig != nil && tt.cluster.Spec.ImportedConfig.PrivateRegistryURL != "" {
				tt.agentImage = image.ResolveWithCluster(tt.agentImage, tt.cluster)
			}

			err := SystemTemplate(&b, &TemplateOps{
				AgentImage:     tt.agentImage,
				AuthImage:      tt.authImage,
				AssetsImage:    tt.assetsImage,
				Namespace:      tt.namespace,
				Token:          tt.token,
				URL:            tt.url,
				IsPreBootstrap: tt.isPreBootstrap,
				Cluster:        tt.cluster,
				AgentFeatures:  tt.features,
				Taints:         tt.taints,
				SecretLister:   secretLister,
				PcExists:       tt.pcExists,
				Mutator:        tt.mutator,
			})

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				return
			}
			assert.NoError(t, err)

			// Snapshot-based assertions
			actual := b.String()
			snapshotFile := filepath.Join(".", "testdata", sanitizeName(tt.name)+".yaml")

			if *update {
				// Write snapshot file
				err := os.MkdirAll(filepath.Dir(snapshotFile), 0755)
				if !assert.NoError(t, err) {
					return
				}
				err = os.WriteFile(snapshotFile, []byte(actual), 0644)
				assert.NoError(t, err)
				return
			}

			// Read expected output
			expected, err := os.ReadFile(snapshotFile)
			if !assert.NoError(t, err, "snapshot file not found: %s", snapshotFile) {
				return
			}

			// Compare
			assert.Equal(t, string(expected), actual)
		})
	}
}

func sanitizeName(name string) string {
	// Convert test name to valid filename
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, ",", "")
	return name
}
