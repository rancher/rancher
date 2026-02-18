package systemtemplate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes/scheme"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	mockSecrets = make(map[string]*corev1.Secret)
)

func resetMockSecrets() {
	mockSecrets = make(map[string]*corev1.Secret)
}

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

		secrets                           map[string]*corev1.Secret
		expectedDeploymentHashes          map[string]string
		expectedDaemonSetHashes           map[string]string
		expectedClusterRoleHashes         map[string]string
		expectedClusterRoleBindingHashes  map[string]string
		expectedNamespaceHashes           map[string]string
		expectedServiceHashes             map[string]string
		expectedServiceAccountHashes      map[string]string
		expectedSecretHashes              map[string]string
		expectedPodDisruptionBudgetHashes map[string]string
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
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "026d73c819a0667fbbd50ca10a4f4215624f5c3d448a1324b24c5f9f8ae99cb3",
			},
			expectedDaemonSetHashes: map[string]string{},
			expectedClusterRoleHashes: map[string]string{
				"proxy-clusterrole-kubeapiserver": "0b1d7f692252b3f498855fa24f669499ba1c061d0ae0eab0db2bb570bc25e63c",
				"cattle-admin":                    "d2b6b43774ce046f3e4e157b94167d6be596d697c3c9411d4ef4d6f29c2d5fde",
			},
			expectedClusterRoleBindingHashes: map[string]string{
				"proxy-role-binding-kubernetes-master": "8e33b2e67243b5a87012489fcd12b4e805c6b6b3c3c2bb4063eee04ca7bc372e",
				"cattle-admin-binding":                 "d646e3b685d8f931a11f4938e4c95a97151286fa391ef03898e6d44f6827cf16",
			},
			expectedNamespaceHashes: map[string]string{
				"cattle-system": "53b1582048d8703999612a3b41f7301b4136e8dd3041d57e9a59c97e76dfa564",
			},
			expectedServiceHashes: map[string]string{
				"cattle-cluster-agent": "03b629bf7287d1a70f31fdf138ea5ec38201040e757b21a808ea0d413e27d65f",
			},
			expectedServiceAccountHashes: map[string]string{
				"cattle": "ba41ec07896a1e2d2319c0ca1405c81faf4ad4c7c0a3c183909860531863202b",
			},
			expectedSecretHashes: map[string]string{
				"cattle-credentials-5ec1f7e700": "38a97eb12e58ccc7ab0b07c8730e0c61fe71f8197aa98ac509431ff265cb2861",
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
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "0ced645edfc4a11bdbf1731fc97ea76c69d5da0f691a395293df4cc6b6ce9e8c",
			},
			expectedDaemonSetHashes: map[string]string{},
			expectedClusterRoleHashes: map[string]string{
				"proxy-clusterrole-kubeapiserver": "0b1d7f692252b3f498855fa24f669499ba1c061d0ae0eab0db2bb570bc25e63c",
				"cattle-admin":                    "d2b6b43774ce046f3e4e157b94167d6be596d697c3c9411d4ef4d6f29c2d5fde",
			},
			expectedClusterRoleBindingHashes: map[string]string{
				"proxy-role-binding-kubernetes-master": "8e33b2e67243b5a87012489fcd12b4e805c6b6b3c3c2bb4063eee04ca7bc372e",
				"cattle-admin-binding":                 "d646e3b685d8f931a11f4938e4c95a97151286fa391ef03898e6d44f6827cf16",
			},
			expectedNamespaceHashes: map[string]string{
				"cattle-system": "53b1582048d8703999612a3b41f7301b4136e8dd3041d57e9a59c97e76dfa564",
			},
			expectedServiceHashes: map[string]string{
				"cattle-cluster-agent": "03b629bf7287d1a70f31fdf138ea5ec38201040e757b21a808ea0d413e27d65f",
			},
			expectedServiceAccountHashes: map[string]string{
				"cattle": "ba41ec07896a1e2d2319c0ca1405c81faf4ad4c7c0a3c183909860531863202b",
			},
			expectedSecretHashes: map[string]string{
				"cattle-credentials-5ec1f7e700": "38a97eb12e58ccc7ab0b07c8730e0c61fe71f8197aa98ac509431ff265cb2861",
			},
			expectedPodDisruptionBudgetHashes: map[string]string{
				"cattle-cluster-agent-pod-disruption-budget": "20b6f53d3abf11951c4cca848ef12e27d3cb46f8f619f2ca2205e2111bc86ee7",
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
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "04e8f9817b3d89a8b7302329bc4447fa70eb43d19051f4bc068bd47e26fa4e61",
			},
			expectedDaemonSetHashes: map[string]string{},
			expectedClusterRoleHashes: map[string]string{
				"proxy-clusterrole-kubeapiserver": "0b1d7f692252b3f498855fa24f669499ba1c061d0ae0eab0db2bb570bc25e63c",
				"cattle-admin":                    "d2b6b43774ce046f3e4e157b94167d6be596d697c3c9411d4ef4d6f29c2d5fde",
			},
			expectedClusterRoleBindingHashes: map[string]string{
				"proxy-role-binding-kubernetes-master": "8e33b2e67243b5a87012489fcd12b4e805c6b6b3c3c2bb4063eee04ca7bc372e",
				"cattle-admin-binding":                 "d646e3b685d8f931a11f4938e4c95a97151286fa391ef03898e6d44f6827cf16",
			},
			expectedNamespaceHashes: map[string]string{
				"cattle-system": "53b1582048d8703999612a3b41f7301b4136e8dd3041d57e9a59c97e76dfa564",
			},
			expectedServiceHashes: map[string]string{
				"cattle-cluster-agent": "03b629bf7287d1a70f31fdf138ea5ec38201040e757b21a808ea0d413e27d65f",
			},
			expectedServiceAccountHashes: map[string]string{
				"cattle": "ba41ec07896a1e2d2319c0ca1405c81faf4ad4c7c0a3c183909860531863202b",
			},
			expectedSecretHashes: map[string]string{
				"cattle-credentials-5ec1f7e700": "38a97eb12e58ccc7ab0b07c8730e0c61fe71f8197aa98ac509431ff265cb2861",
			},
			expectedPodDisruptionBudgetHashes: map[string]string{
				"cattle-cluster-agent-pod-disruption-budget": "20b6f53d3abf11951c4cca848ef12e27d3cb46f8f619f2ca2205e2111bc86ee7",
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
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "b4ffce8a1fc601ce95f599332de597de478a2244fc6bac3b7dc6204416dfb550",
			},
			expectedDaemonSetHashes: map[string]string{},
			expectedClusterRoleHashes: map[string]string{
				"proxy-clusterrole-kubeapiserver": "0b1d7f692252b3f498855fa24f669499ba1c061d0ae0eab0db2bb570bc25e63c",
				"cattle-admin":                    "d2b6b43774ce046f3e4e157b94167d6be596d697c3c9411d4ef4d6f29c2d5fde",
			},
			expectedClusterRoleBindingHashes: map[string]string{
				"proxy-role-binding-kubernetes-master": "8e33b2e67243b5a87012489fcd12b4e805c6b6b3c3c2bb4063eee04ca7bc372e",
				"cattle-admin-binding":                 "d646e3b685d8f931a11f4938e4c95a97151286fa391ef03898e6d44f6827cf16",
			},
			expectedNamespaceHashes: map[string]string{
				"cattle-system": "53b1582048d8703999612a3b41f7301b4136e8dd3041d57e9a59c97e76dfa564",
			},
			expectedServiceHashes: map[string]string{
				"cattle-cluster-agent": "03b629bf7287d1a70f31fdf138ea5ec38201040e757b21a808ea0d413e27d65f",
			},
			expectedServiceAccountHashes: map[string]string{
				"cattle": "ba41ec07896a1e2d2319c0ca1405c81faf4ad4c7c0a3c183909860531863202b",
			},
			expectedSecretHashes: map[string]string{
				"cattle-credentials-d23bc3c633": "17d3bba8f79a57797638bedb21c08c0d0349a27899932cb6e07e107f067b2897",
			},
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
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "5bbbf41e0dcb41ed586e26899d0de6eb474c7d6c309cfac9355a3ee4651b6b3d",
			},
			expectedDaemonSetHashes: map[string]string{},
			expectedClusterRoleHashes: map[string]string{
				"proxy-clusterrole-kubeapiserver": "0b1d7f692252b3f498855fa24f669499ba1c061d0ae0eab0db2bb570bc25e63c",
				"cattle-admin":                    "d2b6b43774ce046f3e4e157b94167d6be596d697c3c9411d4ef4d6f29c2d5fde",
			},
			expectedClusterRoleBindingHashes: map[string]string{
				"proxy-role-binding-kubernetes-master": "8e33b2e67243b5a87012489fcd12b4e805c6b6b3c3c2bb4063eee04ca7bc372e",
				"cattle-admin-binding":                 "d646e3b685d8f931a11f4938e4c95a97151286fa391ef03898e6d44f6827cf16",
			},
			expectedNamespaceHashes: map[string]string{
				"cattle-system": "b759ef69ef6dc6a10cdba8b2d5f2d0635c28eb4a7ceb0f2cd362b906d238b363",
			},
			expectedServiceHashes: map[string]string{
				"cattle-cluster-agent": "03b629bf7287d1a70f31fdf138ea5ec38201040e757b21a808ea0d413e27d65f",
			},
			expectedServiceAccountHashes: map[string]string{
				"cattle": "ba41ec07896a1e2d2319c0ca1405c81faf4ad4c7c0a3c183909860531863202b",
			},
			expectedSecretHashes: map[string]string{
				"cattle-credentials-5ec1f7e700": "38a97eb12e58ccc7ab0b07c8730e0c61fe71f8197aa98ac509431ff265cb2861",
			},
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
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "c7193972923103ae418b609b0fae682322ea9b3b59b00e9a526561d388b2c500",
			},
			expectedDaemonSetHashes: map[string]string{},
			expectedClusterRoleHashes: map[string]string{
				"proxy-clusterrole-kubeapiserver": "0b1d7f692252b3f498855fa24f669499ba1c061d0ae0eab0db2bb570bc25e63c",
				"cattle-admin":                    "d2b6b43774ce046f3e4e157b94167d6be596d697c3c9411d4ef4d6f29c2d5fde",
			},
			expectedClusterRoleBindingHashes: map[string]string{
				"proxy-role-binding-kubernetes-master": "8e33b2e67243b5a87012489fcd12b4e805c6b6b3c3c2bb4063eee04ca7bc372e",
				"cattle-admin-binding":                 "d646e3b685d8f931a11f4938e4c95a97151286fa391ef03898e6d44f6827cf16",
			},
			expectedNamespaceHashes: map[string]string{
				"cattle-system": "c5318858de92544775dc8807b81dc1d68b9481ff01825a9810dc16e795f46246",
			},
			expectedServiceHashes: map[string]string{
				"cattle-cluster-agent": "03b629bf7287d1a70f31fdf138ea5ec38201040e757b21a808ea0d413e27d65f",
			},
			expectedServiceAccountHashes: map[string]string{
				"cattle": "ba41ec07896a1e2d2319c0ca1405c81faf4ad4c7c0a3c183909860531863202b",
			},
			expectedSecretHashes: map[string]string{
				"cattle-credentials-5ec1f7e700": "38a97eb12e58ccc7ab0b07c8730e0c61fe71f8197aa98ac509431ff265cb2861",
			},
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
				Enabled: true,
				Labels: map[string]string{
					"baz": "quz",
				},
			},
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "098ff7fd84702264e219dd843e1e39a73b09535beb08f13c5922e285e5b189cf",
			},
			expectedDaemonSetHashes: map[string]string{},
			expectedClusterRoleHashes: map[string]string{
				"proxy-clusterrole-kubeapiserver": "0b1d7f692252b3f498855fa24f669499ba1c061d0ae0eab0db2bb570bc25e63c",
				"cattle-admin":                    "d2b6b43774ce046f3e4e157b94167d6be596d697c3c9411d4ef4d6f29c2d5fde",
			},
			expectedClusterRoleBindingHashes: map[string]string{
				"proxy-role-binding-kubernetes-master": "8e33b2e67243b5a87012489fcd12b4e805c6b6b3c3c2bb4063eee04ca7bc372e",
				"cattle-admin-binding":                 "d646e3b685d8f931a11f4938e4c95a97151286fa391ef03898e6d44f6827cf16",
			},
			expectedNamespaceHashes: map[string]string{
				"cattle-system": "f44417a05ad2a7421c4726189eab84d74663e21b00b1b6401e969588a87a4431",
			},
			expectedServiceHashes: map[string]string{
				"cattle-cluster-agent": "03b629bf7287d1a70f31fdf138ea5ec38201040e757b21a808ea0d413e27d65f",
			},
			expectedServiceAccountHashes: map[string]string{
				"cattle": "ba41ec07896a1e2d2319c0ca1405c81faf4ad4c7c0a3c183909860531863202b",
			},
			expectedSecretHashes: map[string]string{
				"cattle-credentials-5ec1f7e700": "38a97eb12e58ccc7ab0b07c8730e0c61fe71f8197aa98ac509431ff265cb2861",
			},
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
			defer resetMockSecrets()

			mockSecrets = tt.secrets
			var b bytes.Buffer
			if tt.cluster.Spec.ImportedConfig != nil && tt.cluster.Spec.ImportedConfig.PrivateRegistryURL != "" {
				tt.agentImage = image.ResolveWithCluster(tt.agentImage, tt.cluster)
			}

			err := SystemTemplate(&b, tt.agentImage, tt.authImage, tt.namespace, tt.token, tt.url, tt.isPreBootstrap, tt.cluster, tt.features, tt.taints, secretLister, tt.pcExists, tt.mutator)
			assert.NoError(t, err)

			decoder := scheme.Codecs.UniversalDeserializer()
			for _, r := range strings.Split(b.String(), "---") {
				if len(r) == 0 {
					continue
				}

				obj, groupVersionKind, err := decoder.Decode(
					[]byte(r),
					nil,
					nil)
				if err != nil {
					continue
				}

				switch groupVersionKind.Kind {
				case "Deployment":
					deployment := obj.(*appsv1.Deployment)
					b, err := json.Marshal(deployment)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					assert.Equal(t, tt.expectedDeploymentHashes[deployment.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, deployment.Name))
				case "ClusterRole":
					clusterrole := obj.(*rbacv1.ClusterRole)
					b, err := json.Marshal(clusterrole)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					assert.Equal(t, tt.expectedClusterRoleHashes[clusterrole.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, clusterrole.Name))
				case "ClusterRoleBinding":
					crb := obj.(*rbacv1.ClusterRoleBinding)
					b, err := json.Marshal(crb)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					assert.Equal(t, tt.expectedClusterRoleBindingHashes[crb.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, crb.Name))
				case "Namespace":
					ns := obj.(*corev1.Namespace)
					b, err := json.Marshal(ns)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					assert.Equal(t, tt.expectedNamespaceHashes[ns.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, ns.Name))
				case "DaemonSet":
					ds := obj.(*appsv1.DaemonSet)
					b, err := json.Marshal(ds)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					assert.Equal(t, tt.expectedDaemonSetHashes[ds.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, ds.Name))
				case "Service":
					svc := obj.(*corev1.Service)
					b, err := json.Marshal(svc)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					assert.Equal(t, tt.expectedServiceHashes[svc.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, svc.Name))
				case "ServiceAccount":
					svcacct := obj.(*corev1.ServiceAccount)
					b, err := json.Marshal(svcacct)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					assert.Equal(t, tt.expectedServiceAccountHashes[svcacct.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, svcacct.Name))
				case "Secret":
					secret := obj.(*corev1.Secret)
					b, err := json.Marshal(secret)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					assert.Equal(t, tt.expectedSecretHashes[secret.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, secret.Name))
				case "PodDisruptionBudget":
					pdb := obj.(*policyv1.PodDisruptionBudget)
					b, err := json.Marshal(pdb)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					assert.Equal(t, tt.expectedPodDisruptionBudgetHashes[pdb.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, pdb.Name))
				default:
					assert.FailNow(t, fmt.Sprintf("unexpected Kind for GVK: %s", groupVersionKind.String()))
				}
			}
		})
	}
}

func getHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
