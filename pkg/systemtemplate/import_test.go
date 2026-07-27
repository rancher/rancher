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
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "4578d1aabe41fea8ab4855f0db1a69a79b9244301931e6d8553260c9b16b8e25",
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
				"cattle-cluster-agent": "da5a4c95e8c3f7b88a53cff480e760a080682f53acfc015eec73d3284f924a9e",
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
				"cattle-cluster-agent": "c0ecf4b19d65c3e3fe63935566110b8e7609692812db348f16a3fb25b6292555",
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
				"cattle-cluster-agent": "6b10c91174736f1a18937afe25caebc3f7c981f0c054e298cd738f5cf1c2492c",
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
			agentImage: "my/agent:image",
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "806f0ad4c927b94146ce90ea300456b16f12eacd667c1b6f8fdb3f390986f04e",
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
			agentImage: "my/agent:image",
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "ab9d55a0e310e89613e4dc605422a9b9d8a8df47862ec24236ceb78793d226f3",
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
				Enabled:     true,
				Annotations: map[string]string{},
				Labels: map[string]string{
					"baz": "quz",
				},
			},
			agentImage: "my/agent:image",
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "d7b63bf194eb35ec380ed255697f80233709414003bc8e89a6224cbcc1a2a542",
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
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "1f12809ec7e599f682bf30f25e19a2688a6f0b38359d2479197109926290fa0a",
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
				"cattle-credentials-a15c1b308a":              "e5b7eedd320bfd99546203b85a24d29b6be73e506053ac39065fd42ebb217b86",
				"my-pull-secret-rancher-managed-pull-secret": "2df73d5a6af032f3ac7014a342b7f8e36a2ddc6431be33f5ffe03964fa17f8fc",
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
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "2c3d5e42a2c0d0395d8f89ccc2880be1ce4152bf83e7af4164fb0d0b64f1cb1a",
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
				"cattle-credentials-a15c1b308a": "e5b7eedd320bfd99546203b85a24d29b6be73e506053ac39065fd42ebb217b86",
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
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "9786f072937ec0fcedb307d5bdc14814bd93b97ce34d2ed7cea12526bb5ed6c9",
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
				"cattle-credentials-a15c1b308a":          "e5b7eedd320bfd99546203b85a24d29b6be73e506053ac39065fd42ebb217b86",
				"secret-one-rancher-managed-pull-secret": "d4e2a71ad3344fd5487e36c6f9987bdad155e42e75675f0af39f9b03027203a1",
				"secret-two-rancher-managed-pull-secret": "a8b13caa20117d26374c99ffbc3dbc4b9a62d9b58c4040b43ae11596c1fe7b0c",
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
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent-bootstrap": "69ba65298a3697e1375fc8fbc292712e8161d38fde14a7449b4e6c078b43405f",
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
				"cattle-credentials-a15c1b308a": "e5b7eedd320bfd99546203b85a24d29b6be73e506053ac39065fd42ebb217b86",
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

			// Hash-based assertions
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
					t.Logf("Hash %s/%s: %s", groupVersionKind.Kind, deployment.Name, getHash(b))
					if tt.expectedDeploymentHashes != nil {
						assert.Equal(t, tt.expectedDeploymentHashes[deployment.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, deployment.Name))
					}
				case "ClusterRole":
					clusterrole := obj.(*rbacv1.ClusterRole)
					b, err := json.Marshal(clusterrole)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					t.Logf("Hash %s/%s: %s", groupVersionKind.Kind, clusterrole.Name, getHash(b))
					if tt.expectedClusterRoleHashes != nil {
						assert.Equal(t, tt.expectedClusterRoleHashes[clusterrole.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, clusterrole.Name))
					}
				case "ClusterRoleBinding":
					crb := obj.(*rbacv1.ClusterRoleBinding)
					b, err := json.Marshal(crb)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					t.Logf("Hash %s/%s: %s", groupVersionKind.Kind, crb.Name, getHash(b))
					if tt.expectedClusterRoleBindingHashes != nil {
						assert.Equal(t, tt.expectedClusterRoleBindingHashes[crb.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, crb.Name))
					}
				case "Namespace":
					ns := obj.(*corev1.Namespace)
					b, err := json.Marshal(ns)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					t.Logf("Hash %s/%s: %s", groupVersionKind.Kind, ns.Name, getHash(b))
					if tt.expectedNamespaceHashes != nil {
						assert.Equal(t, tt.expectedNamespaceHashes[ns.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, ns.Name))
					}
				case "DaemonSet":
					ds := obj.(*appsv1.DaemonSet)
					b, err := json.Marshal(ds)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					t.Logf("Hash %s/%s: %s", groupVersionKind.Kind, ds.Name, getHash(b))
					if tt.expectedDaemonSetHashes != nil {
						assert.Equal(t, tt.expectedDaemonSetHashes[ds.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, ds.Name))
					}
				case "Service":
					svc := obj.(*corev1.Service)
					b, err := json.Marshal(svc)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					t.Logf("Hash %s/%s: %s", groupVersionKind.Kind, svc.Name, getHash(b))
					if tt.expectedServiceHashes != nil {
						assert.Equal(t, tt.expectedServiceHashes[svc.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, svc.Name))
					}
				case "ServiceAccount":
					svcacct := obj.(*corev1.ServiceAccount)
					b, err := json.Marshal(svcacct)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					t.Logf("Hash %s/%s: %s", groupVersionKind.Kind, svcacct.Name, getHash(b))
					if tt.expectedServiceAccountHashes != nil {
						assert.Equal(t, tt.expectedServiceAccountHashes[svcacct.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, svcacct.Name))
					}
				case "Secret":
					secret := obj.(*corev1.Secret)
					b, err := json.Marshal(secret)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					t.Logf("Hash %s/%s: %s", groupVersionKind.Kind, secret.Name, getHash(b))
					if tt.expectedSecretHashes != nil {
						assert.Equal(t, tt.expectedSecretHashes[secret.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, secret.Name))
					}
				case "PodDisruptionBudget":
					pdb := obj.(*policyv1.PodDisruptionBudget)
					b, err := json.Marshal(pdb)
					if err != nil {
						assert.FailNow(t, err.Error())
					}
					t.Logf("Hash %s/%s: %s", groupVersionKind.Kind, pdb.Name, getHash(b))
					if tt.expectedPodDisruptionBudgetHashes != nil {
						assert.Equal(t, tt.expectedPodDisruptionBudgetHashes[pdb.Name], getHash(b), fmt.Sprintf("%s/%s", groupVersionKind.Kind, pdb.Name))
					}
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
