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
			url:        "some-dummy-url",
			token:      "some-dummy-token",
			agentImage: "my/agent:image",
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
			agentImage: "my/agent:image",
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
			agentImage: "my/agent:image",
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
			agentImage: "my/agent:image",
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
			agentImage: "rancher/rancher-agent:v2.8.0",
			token:      "test-token",
			url:        "https://rancher.example.com",
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
			agentImage: "rancher-agent:v2.8.0",
			token:      "test-token",
			url:        "https://rancher.example.com",
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
			agentImage: "rancher-agent:v2.8.0",
			token:      "test-token",
			url:        "https://rancher.example.com",
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
			token:          "test-token",
			url:            "https://rancher.example.com",
			isPreBootstrap: true,
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
