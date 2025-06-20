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
	rketypes "github.com/rancher/rke/types"
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
		name                              string
		cluster                           *apimgmtv3.Cluster
		pcExists                          bool
		agentImage                        string
		authImage                         string
		namespace                         string
		token                             string
		url                               string
		isPreBootstrap                    bool
		features                          map[string]bool
		taints                            []corev1.Taint
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
				"cattle-cluster-agent": "9a35d9bed78e5c35b2ae9bcedbc60d72eb4201d817674cb60c222a1c77795cb4",
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
				"cattle-credentials-5ec1f7e700": "b2ec2a5655ff908557b5a46695be7429c9eb5bf32c799e37832e57405ce54f46",
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
				"cattle-cluster-agent": "9a35d9bed78e5c35b2ae9bcedbc60d72eb4201d817674cb60c222a1c77795cb4",
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
				"cattle-credentials-5ec1f7e700": "b2ec2a5655ff908557b5a46695be7429c9eb5bf32c799e37832e57405ce54f46",
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
			},
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "9a35d9bed78e5c35b2ae9bcedbc60d72eb4201d817674cb60c222a1c77795cb4",
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
				"cattle-credentials-5ec1f7e700": "b2ec2a5655ff908557b5a46695be7429c9eb5bf32c799e37832e57405ce54f46",
			},
			expectedPodDisruptionBudgetHashes: map[string]string{
				"cattle-cluster-agent-pod-disruption-budget": "89bb4831cc31587904eaacb4d6d395b33984c8940b7a3e71e3066788a805e483",
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
			},
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "2093af1547b1eef8b22527ecb1590fe3302c3287b317e0aed3e268ff2a6dc0df",
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
				"cattle-credentials-5ec1f7e700": "b2ec2a5655ff908557b5a46695be7429c9eb5bf32c799e37832e57405ce54f46",
			},
			expectedPodDisruptionBudgetHashes: map[string]string{
				"cattle-cluster-agent-pod-disruption-budget": "89bb4831cc31587904eaacb4d6d395b33984c8940b7a3e71e3066788a805e483",
			},
		},
		{
			name: "test-provisioned-import-custom-agent",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-prov",
				},
				Spec: apimgmtv3.ClusterSpec{
					ImportedConfig: &apimgmtv3.ImportedConfig{
						PrivateRegistryURL: "localhost:5001",
					},
				},
			},
			url:        "some-dummy-url",
			token:      "some-dummy-token",
			agentImage: "my/agent:image",
			expectedDeploymentHashes: map[string]string{
				"cattle-cluster-agent": "48868d8d72bfe924be0eb248def83f78088c68f91e12cd2b67116e6d448d889a",
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
				"cattle-credentials-d23bc3c633": "3d45d058d965019241aca1fe9f9658aafab23acd85466569dab40414611f6fa7",
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
			err := SystemTemplate(&b, tt.agentImage, tt.authImage, tt.namespace, tt.token, tt.url, tt.isPreBootstrap, tt.cluster, tt.features, tt.taints, secretLister, tt.pcExists)

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
