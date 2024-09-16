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
	rketypes "github.com/rancher/rke/types"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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

	tests := []struct {
		name                             string
		cluster                          *apimgmtv3.Cluster
		agentImage                       string
		authImage                        string
		namespace                        string
		token                            string
		url                              string
		isWindowsCluster                 bool
		isPreBootstrap                   bool
		features                         map[string]bool
		taints                           []corev1.Taint
		secrets                          map[string]*corev1.Secret
		expectedDeploymentHashes         map[string]string
		expectedDaemonSetHashes          map[string]string
		expectedClusterRoleHashes        map[string]string
		expectedClusterRoleBindingHashes map[string]string
		expectedNamespaceHashes          map[string]string
		expectedServiceHashes            map[string]string
		expectedServiceAccountHashes     map[string]string
		expectedSecretHashes             map[string]string
	}{
		{
			name: "test-rke",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-rke",
				},
				Spec: apimgmtv3.ClusterSpec{
					ClusterSpecBase: apimgmtv3.ClusterSpecBase{
						RancherKubernetesEngineConfig: &rketypes.RancherKubernetesEngineConfig{},
					},
				},
			},
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "734cc6a47dfa564f230d39da48782e6f2ba9a55e0385aaa4d2fa7405375d8527",
			},
			expectedDaemonSetHashes: map[string]string{},
			expectedClusterRoleHashes: map[string]string{
				"proxy-clusterrole-kubeapiserver": "0d28ae2947ce0c5faef85ff59169a5f65e0490552bf9cb00f29a98eb97a02a7e",
				"cattle-admin":                    "009abecc023b1e4ac1bc35e4153ef4492b2bc66a5972df9c5617a38f587c3f42",
			},
			expectedClusterRoleBindingHashes: map[string]string{
				"proxy-role-binding-kubernetes-master": "0df909395597974e60d905e9860bc0a02367bd2df74528d430c635c3f7afdeb0",
				"cattle-admin-binding":                 "0da37cf0d4c4b4d068a3000967c4e37d11e1cecd126779633095dbe30b39c6ba",
			},
			expectedNamespaceHashes: map[string]string{
				"cattle-system": "fd527fed9cae2e8b27f9610d64e9476e692a3dfde42954aeaecba450fe2b9571",
			},
			expectedServiceHashes: map[string]string{
				"cattle-cluster-agent": "9512a8430f6d32f31eac6e4446724dc5a336c3d9c8147c824f2734c2f8afe792",
			},
			expectedServiceAccountHashes: map[string]string{
				"cattle": "5cf160de85eaef5de9ce917130c64c23e91836920f7e9b2e2d7a8be8290079f2",
			},
			expectedSecretHashes: map[string]string{
				"cattle-credentials-d41d8cd": "131d05388e50e23e5f22eb3b54676910e6ded959b3dd1333f7bc2096ee2e95e9",
			},
		},
		{
			name: "test-provisioned-import",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-prov",
				},
				Spec: apimgmtv3.ClusterSpec{
					ImportedConfig: &apimgmtv3.ImportedConfig{},
				},
			},
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "734cc6a47dfa564f230d39da48782e6f2ba9a55e0385aaa4d2fa7405375d8527",
			},
			expectedDaemonSetHashes: map[string]string{},
			expectedClusterRoleHashes: map[string]string{
				"proxy-clusterrole-kubeapiserver": "0d28ae2947ce0c5faef85ff59169a5f65e0490552bf9cb00f29a98eb97a02a7e",
				"cattle-admin":                    "009abecc023b1e4ac1bc35e4153ef4492b2bc66a5972df9c5617a38f587c3f42",
			},
			expectedClusterRoleBindingHashes: map[string]string{
				"proxy-role-binding-kubernetes-master": "0df909395597974e60d905e9860bc0a02367bd2df74528d430c635c3f7afdeb0",
				"cattle-admin-binding":                 "0da37cf0d4c4b4d068a3000967c4e37d11e1cecd126779633095dbe30b39c6ba",
			},
			expectedNamespaceHashes: map[string]string{
				"cattle-system": "fd527fed9cae2e8b27f9610d64e9476e692a3dfde42954aeaecba450fe2b9571",
			},
			expectedServiceHashes: map[string]string{
				"cattle-cluster-agent": "9512a8430f6d32f31eac6e4446724dc5a336c3d9c8147c824f2734c2f8afe792",
			},
			expectedServiceAccountHashes: map[string]string{
				"cattle": "5cf160de85eaef5de9ce917130c64c23e91836920f7e9b2e2d7a8be8290079f2",
			},
			expectedSecretHashes: map[string]string{
				"cattle-credentials-d41d8cd": "131d05388e50e23e5f22eb3b54676910e6ded959b3dd1333f7bc2096ee2e95e9",
			},
		},
		{
			name: "test-provisioned-import-custom-agent",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-prov",
				},
				Spec: apimgmtv3.ClusterSpec{
					ImportedConfig: &apimgmtv3.ImportedConfig{},
				},
			},
			url:        "some-dummy-url",
			token:      "some-dummy-token",
			agentImage: "my/agent:image",
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "1d9554ad0e8dda26a8e4fa96879a5954a478bf9b22e2b1de4273292774390226",
			},
			expectedDaemonSetHashes: map[string]string{},
			expectedClusterRoleHashes: map[string]string{
				"proxy-clusterrole-kubeapiserver": "0d28ae2947ce0c5faef85ff59169a5f65e0490552bf9cb00f29a98eb97a02a7e",
				"cattle-admin":                    "009abecc023b1e4ac1bc35e4153ef4492b2bc66a5972df9c5617a38f587c3f42",
			},
			expectedClusterRoleBindingHashes: map[string]string{
				"proxy-role-binding-kubernetes-master": "0df909395597974e60d905e9860bc0a02367bd2df74528d430c635c3f7afdeb0",
				"cattle-admin-binding":                 "0da37cf0d4c4b4d068a3000967c4e37d11e1cecd126779633095dbe30b39c6ba",
			},
			expectedNamespaceHashes: map[string]string{
				"cattle-system": "fd527fed9cae2e8b27f9610d64e9476e692a3dfde42954aeaecba450fe2b9571",
			},
			expectedServiceHashes: map[string]string{
				"cattle-cluster-agent": "9512a8430f6d32f31eac6e4446724dc5a336c3d9c8147c824f2734c2f8afe792",
			},
			expectedServiceAccountHashes: map[string]string{
				"cattle": "5cf160de85eaef5de9ce917130c64c23e91836920f7e9b2e2d7a8be8290079f2",
			},
			expectedSecretHashes: map[string]string{
				"cattle-credentials-ea6f059": "13abfa9516b89b23f9451a71c3258a358ab68abddd6d9b661a106dc762028ada",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetMockSecrets()

			mockSecrets = tt.secrets
			var b bytes.Buffer
			err := SystemTemplate(&b, tt.agentImage, tt.authImage, tt.namespace, tt.token, tt.url, tt.isWindowsCluster, tt.isPreBootstrap, tt.cluster, tt.features, tt.taints, secretLister)

			assert.Nil(t, err)
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
