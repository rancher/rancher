package cluster

import (
	"encoding/base64"
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator/assemblers"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	rketypes "github.com/rancher/rke/types"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// It is not currently possible to test ECR credentials, because they require valid credentials and communicate with
// the ecr service to generate an auth config.
func TestGeneratePrivateRegistryDockerConfig(t *testing.T) {
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
		name           string
		expectedUrl    string
		expectedConfig string
		expectedError  string
		cluster        *v3.Cluster
		secrets        []*corev1.Secret
	}{
		{
			name:           "nil",
			expectedUrl:    "",
			expectedConfig: "",
			expectedError:  "",
			cluster:        nil,
		},
		{
			name:           "rke1 private registry",
			expectedUrl:    "0123456789abcdef.dkr.ecr.us-east-1.amazonaws.com",
			expectedConfig: base64.URLEncoding.EncodeToString([]byte("testConfig")), // should be directly copied from RKE1 secret
			expectedError:  "",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterSecrets: v3.ClusterSecrets{
							PrivateRegistrySecret: "test-secret",
						},
						RancherKubernetesEngineConfig: &rketypes.RancherKubernetesEngineConfig{
							PrivateRegistries: []rketypes.PrivateRegistry{
								{
									URL:  "0123456789abcdef.dkr.ecr.us-east-1.amazonaws.com",
									User: "testuser",
								},
							},
						},
					},
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: assemblers.SecretNamespace,
						Name:      "test-secret",
					},
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte("testConfig"),
					},
				},
			},
		},
		{
			name:           "v2prov private registry",
			expectedUrl:    "0123456789abcdef.dkr.ecr.us-east-1.amazonaws.com",
			expectedConfig: base64.URLEncoding.EncodeToString([]byte(`{"auths":{"0123456789abcdef.dkr.ecr.us-east-1.amazonaws.com":{"username":"testuser","password":"password","auth":"dGVzdHVzZXI6cGFzc3dvcmQ="}}}`)),
			expectedError:  "",
			cluster: &v3.Cluster{
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterSecrets: v3.ClusterSecrets{
							PrivateRegistrySecret: "test-secret",
							PrivateRegistryURL:    "0123456789abcdef.dkr.ecr.us-east-1.amazonaws.com",
						},
					},
					FleetWorkspaceName: "fleet-default",
				},
			},
			secrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "fleet-default",
						Name:      "test-secret",
					},
					Data: map[string][]byte{
						"username": []byte("testuser"),
						"password": []byte("password"),
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
			url, cfg, err := GeneratePrivateRegistryEncodedDockerConfig(tt.cluster, secretLister)
			assert.Equal(t, tt.expectedUrl, url)
			assert.Equal(t, tt.expectedConfig, cfg)
			if tt.expectedError == "" {
				assert.Nil(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedError)
			}
		})
	}
}
