package secretmigrator

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator/assemblers"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	v3fakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	rketypes "github.com/rancher/rke/types"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	configv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

var (
	mockSecrets  = make(map[string]*corev1.Secret)
	mockClusters = make(map[string]*apimgmtv3.Cluster)
)

const (
	secretsNS = "cattle-global-data"
)

func resetMockSecrets() {
	mockSecrets = make(map[string]*corev1.Secret)
}

func resetMockClusters() {
	mockClusters = make(map[string]*apimgmtv3.Cluster)
}

func newTestHandler(t *testing.T) *handler {
	secrets := corefakes.SecretInterfaceMock{
		CreateFunc: func(secret *corev1.Secret) (*corev1.Secret, error) {
			if secret.GenerateName != "" {
				uniqueIdentifier := md5.Sum([]byte(time.Now().String()))
				secret.Name = secret.GenerateName + hex.EncodeToString(uniqueIdentifier[:])[:5]
				secret.GenerateName = ""
			}
			// All key-value pairs in the stringData field are internally merged into the data field.
			// If a key appears in both the data and the stringData field, the value specified in the stringData field takes
			// precedence.
			// https://kubernetes.io/docs/concepts/configuration/secret/#restriction-names-data
			// All keys and values are merged into the data field on write, overwriting any existing values.
			// The stringData field is never output when reading from the API.
			// https://pkg.go.dev/k8s.io/api/core/v1@v0.24.2#Secret.StringData
			if secret.StringData != nil && len(secret.StringData) != 0 {
				if secret.Data == nil {
					secret.Data = map[string][]byte{}
				}
				for k, v := range secret.StringData {
					secret.Data[k] = []byte(v)
				}
			}
			secret.ResourceVersion = "0"
			secret.StringData = map[string]string{}
			key := fmt.Sprintf("%s:%s", secret.Namespace, secret.Name)
			mockSecrets[key] = secret.DeepCopy()
			return mockSecrets[key], nil
		},
		UpdateFunc: func(secret *corev1.Secret) (*corev1.Secret, error) {
			key := fmt.Sprintf("%s:%s", secret.Namespace, secret.Name)
			if _, ok := mockSecrets[key]; !ok {
				return nil, apierror.NewNotFound(schema.GroupResource{}, fmt.Sprintf("secret [%s] not found", key))
			}

			if secret.StringData != nil && len(secret.StringData) != 0 {
				for k, v := range secret.StringData {
					secret.Data[k] = []byte(v)
				}
			}
			secret.StringData = map[string]string{}
			rv, _ := strconv.Atoi(mockSecrets[key].ObjectMeta.ResourceVersion)
			rv++
			if reflect.DeepEqual(secret, mockSecrets[key]) {
				assert.Fail(t, "update called with no changes")
			}
			secret.ResourceVersion = strconv.Itoa(rv)
			mockSecrets[key] = secret
			return mockSecrets[key].DeepCopy(), nil
		},
		DeleteNamespacedFunc: func(namespace string, name string, options *metav1.DeleteOptions) error {
			key := fmt.Sprintf("%s:%s", namespace, name)
			if _, ok := mockSecrets[key]; !ok {
				return apierror.NewNotFound(schema.GroupResource{}, fmt.Sprintf("secret [%s] not found", key))
			}
			mockSecrets[fmt.Sprintf("%s:%s", namespace, name)] = nil
			return nil
		},
	}

	secretLister := corefakes.SecretListerMock{
		GetFunc: func(namespace string, name string) (*corev1.Secret, error) {
			id := fmt.Sprintf("%s:%s", namespace, name)
			secret, ok := mockSecrets[fmt.Sprintf("%s:%s", namespace, name)]
			if !ok {
				return nil, apierror.NewNotFound(schema.GroupResource{}, id)
			}
			return secret.DeepCopy(), nil
		},
	}

	notifierLister := &v3fakes.NotifierListerMock{
		ListFunc: func(namespace string, selector labels.Selector) ([]*apimgmtv3.Notifier, error) {
			var list []*apimgmtv3.Notifier
			return list, nil
		},
	}

	clusterCatalogLister := &v3fakes.ClusterCatalogListerMock{
		ListFunc: func(namespace string, selector labels.Selector) ([]*apimgmtv3.ClusterCatalog, error) {
			var list []*apimgmtv3.ClusterCatalog
			return list, nil
		},
	}

	projectLister := &v3fakes.ProjectListerMock{
		ListFunc: func(namespace string, selector labels.Selector) ([]*apimgmtv3.Project, error) {
			var list []*apimgmtv3.Project
			return list, nil
		},
	}
	return &handler{
		clusters: &v3fakes.ClusterInterfaceMock{
			CreateFunc: func(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
				mockClusters[cluster.Name] = cluster.DeepCopy()
				mockClusters[cluster.Name].ObjectMeta.ResourceVersion = "0"
				return mockClusters[cluster.Name], nil
			},
			UpdateFunc: func(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
				if _, ok := mockClusters[cluster.Name]; !ok {
					return nil, apierror.NewNotFound(schema.GroupResource{}, fmt.Sprintf("cluster [%s]", cluster.Name))
				}
				if reflect.DeepEqual(mockClusters[cluster.Name], cluster) {
					assert.Fail(t, "update called with no changes")
				}
				mockClusters[cluster.Name] = cluster.DeepCopy()
				rv, _ := strconv.Atoi(mockClusters[cluster.Name].ObjectMeta.ResourceVersion)
				rv++
				mockClusters[cluster.Name].ObjectMeta.ResourceVersion = strconv.Itoa(rv)
				return mockClusters[cluster.Name].DeepCopy(), nil
			},
			GetFunc: func(name string, opts metav1.GetOptions) (*apimgmtv3.Cluster, error) {
				cluster, ok := mockClusters[name]
				if !ok {
					gvk := cluster.GroupVersionKind()
					return nil, apierror.NewNotFound(schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, name)
				}
				return cluster.DeepCopy(), nil
			},
		},
		migrator:             NewMigrator(&secretLister, &secrets),
		notifierLister:       notifierLister,
		clusterCatalogLister: clusterCatalogLister,
		projectLister:        projectLister,
	}
}

func TestMigrateClusterSecrets(t *testing.T) {
	h := newTestHandler(t)
	defer resetMockClusters()
	defer resetMockSecrets()
	secretKey := "abcdefg123"
	testCluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testcluster",
		},
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				RancherKubernetesEngineConfig: &rketypes.RancherKubernetesEngineConfig{
					PrivateRegistries: []rketypes.PrivateRegistry{
						{
							URL:      "testurl",
							Password: secretKey,
						},
					},
					Services: rketypes.RKEConfigServices{
						Etcd: rketypes.ETCDService{
							BackupConfig: &rketypes.BackupConfig{
								S3BackupConfig: &rketypes.S3BackupConfig{
									SecretKey: secretKey,
								},
							},
						},
					},
					Network: rketypes.NetworkConfig{
						WeaveNetworkProvider: &rketypes.WeaveNetworkProvider{
							Password: secretKey,
						},
					},
					CloudProvider: rketypes.CloudProvider{
						VsphereCloudProvider: &rketypes.VsphereCloudProvider{
							Global: rketypes.GlobalVsphereOpts{
								Password: secretKey,
							},
							VirtualCenter: map[string]rketypes.VirtualCenterConfig{
								"vc1": {
									Password: secretKey,
								},
							},
						},
						OpenstackCloudProvider: &rketypes.OpenstackCloudProvider{
							Global: rketypes.GlobalOpenstackOpts{
								Password: secretKey,
							},
						},
						AzureCloudProvider: &rketypes.AzureCloudProvider{
							AADClientSecret:       secretKey,
							AADClientCertPassword: secretKey,
						},
					},
				},
			},
		},
	}
	testCluster, err := h.clusters.Create(testCluster)
	assert.Nil(t, err)
	cluster, err := h.migrateClusterSecrets(testCluster)
	assert.Nil(t, err)

	registry := credentialprovider.DockerConfigJSON{
		Auths: credentialprovider.DockerConfig{
			"testurl": credentialprovider.DockerConfigEntry{
				Password: secretKey,
			},
		},
	}

	registryJSON, err := json.Marshal(registry)
	assert.Nil(t, err)

	tests := []struct {
		name       string
		field      string
		secretName string
		key        string
		expected   string
	}{
		{
			name:       "privateRegistry",
			field:      cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries[0].Password,
			secretName: cluster.Spec.ClusterSecrets.PrivateRegistrySecret,
			key:        corev1.DockerConfigJsonKey,
			expected:   string(registryJSON),
		},
		{
			name:       "s3Secret",
			field:      cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey,
			secretName: cluster.Spec.ClusterSecrets.S3CredentialSecret,
			key:        SecretKey,
			expected:   secretKey,
		},
		{
			name:       "WeavePassword",
			field:      cluster.Spec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password,
			secretName: cluster.Spec.ClusterSecrets.WeavePasswordSecret,
			key:        SecretKey,
			expected:   secretKey,
		},
		{
			name:       "VspherePassword",
			field:      cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.Global.Password,
			secretName: cluster.Spec.ClusterSecrets.VsphereSecret,
			key:        SecretKey,
			expected:   secretKey,
		},
		{
			name:       "VirtualCenterPassword",
			field:      cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter["vc1"].Password,
			secretName: cluster.Spec.ClusterSecrets.VirtualCenterSecret,
			key:        "vc1",
			expected:   secretKey,
		},
		{
			name:       "OpenStackPassword",
			field:      cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.OpenstackCloudProvider.Global.Password,
			secretName: cluster.Spec.ClusterSecrets.OpenStackSecret,
			key:        SecretKey,
			expected:   secretKey,
		},
		{
			name:       "AADClientSecret",
			field:      cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientSecret,
			secretName: cluster.Spec.ClusterSecrets.AADClientSecret,
			key:        SecretKey,
			expected:   secretKey,
		},
		{
			name:       "AADClientCertSecret",
			field:      cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientCertPassword,
			secretName: cluster.Spec.ClusterSecrets.AADClientCertSecret,
			key:        SecretKey,
			expected:   secretKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, "", tt.field)
			assert.NotEqual(t, "", tt.secretName)
			secret, err := h.migrator.secretLister.Get(secretsNS, tt.secretName)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, string(secret.Data[tt.key]))
		})
	}

	assert.True(t, apimgmtv3.ClusterConditionSecretsMigrated.IsTrue(cluster))

	// test that cluster does not get updated if migrated again
	clusterCopy := cluster.DeepCopy()
	clusterCopy, err = h.migrateClusterSecrets(clusterCopy)
	assert.Nil(t, err)
	assert.Equal(t, cluster, clusterCopy)

	testCluster2 := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testcluster2",
		},
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				RancherKubernetesEngineConfig: &rketypes.RancherKubernetesEngineConfig{
					Services: rketypes.RKEConfigServices{
						Etcd: rketypes.ETCDService{
							BackupConfig: &rketypes.BackupConfig{
								S3BackupConfig: &rketypes.S3BackupConfig{
									SecretKey: secretKey,
								},
							},
						},
					},
				},
			},
		},
	}
	cluster, err = h.migrateClusterSecrets(testCluster2)
	assert.Equal(t, err.Error(), fmt.Sprintf(" \"cluster [%s]\" not found", testCluster2.Name))
	// no change should
	assert.Equal(t, cluster.Spec.RancherKubernetesEngineConfig, testCluster2.Spec.RancherKubernetesEngineConfig)
	assert.Equal(t, cluster, testCluster2)
	assert.True(t, apimgmtv3.ClusterConditionSecretsMigrated.IsFalse(cluster))
}

func TestMigrateClusterServiceAccountToken(t *testing.T) {
	h := newTestHandler(t)
	defer resetMockClusters()
	defer resetMockSecrets()
	token := "somefaketoken"

	testCluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testcluster",
		},
		Status: apimgmtv3.ClusterStatus{
			ServiceAccountToken: token,
		},
	}
	_, err := h.clusters.Create(testCluster)
	assert.Nil(t, err)
	cluster, err := h.migrateServiceAccountSecrets(testCluster)
	assert.Nil(t, err)
	assert.Equal(t, cluster.Status.ServiceAccountToken, "")

	secretName := cluster.Status.ServiceAccountTokenSecret
	assert.NotEqual(t, secretName, "")
	secret, err := h.migrator.secretLister.Get(secretsNS, secretName)
	assert.Nil(t, err)
	assert.Equal(t, secret.Data[SecretKey], []byte(token))
	assert.True(t, apimgmtv3.ClusterConditionServiceAccountSecretsMigrated.IsTrue(cluster))

	// test that cluster object does not get updated if migrated again
	clusterCopy := cluster.DeepCopy()
	clusterCopy, err = h.migrateServiceAccountSecrets(clusterCopy)
	assert.Nil(t, err)
	assert.Equal(t, cluster, clusterCopy) // purposefully test pointer equality
}

func TestMigrateRKESecrets(t *testing.T) {
	h := newTestHandler(t)
	defer resetMockClusters()
	defer resetMockSecrets()

	testCluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testcluster",
		},
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				RancherKubernetesEngineConfig: &rketypes.RancherKubernetesEngineConfig{
					BastionHost: rketypes.BastionHost{
						SSHKey: "sshKey",
					},
					PrivateRegistries: []rketypes.PrivateRegistry{
						{
							URL: "testurl",
							ECRCredentialPlugin: &rketypes.ECRCredentialPlugin{
								AwsAccessKeyID:     "keyId",
								AwsSecretAccessKey: "secret",
								AwsSessionToken:    "token",
							},
						},
					},
					Services: rketypes.RKEConfigServices{
						Kubelet: rketypes.KubeletService{
							BaseService: rketypes.BaseService{
								ExtraEnv: []string{
									"AWS_ACCESS_KEY_ID=keyId",
									"AWS_SECRET_ACCESS_KEY=secret",
								},
							},
						},
						KubeAPI: rketypes.KubeAPIService{
							SecretsEncryptionConfig: &rketypes.SecretsEncryptionConfig{
								CustomConfig: &configv1.EncryptionConfiguration{
									Resources: []configv1.ResourceConfiguration{
										{
											Providers: []configv1.ProviderConfiguration{
												{
													AESGCM: &configv1.AESConfiguration{
														Keys: []configv1.Key{
															{
																Name:   "testName",
																Secret: "testSecret",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	_, err := h.clusters.Create(testCluster)
	assert.Nil(t, err)
	cluster, err := h.migrateRKESecrets(testCluster)
	assert.Nil(t, err)

	// test that cluster object does not get updated if migrated again
	clusterCopy := cluster.DeepCopy()
	clusterCopy, err = h.migrateRKESecrets(clusterCopy)
	assert.Nil(t, err)
	assert.Equal(t, cluster, clusterCopy)

	type verifyFunc func(t *testing.T, cluster *apimgmtv3.Cluster)

	emptyStringCondition := func(fields ...string) verifyFunc {
		return func(t *testing.T, cluster *apimgmtv3.Cluster) {
			for _, field := range fields {
				assert.Equal(t, "", field)
			}
		}
	}

	tests := []struct {
		name               string
		cleanupVerifyFunc  verifyFunc
		secretName         string
		key                string
		expected           string
		assembler          assemblers.Assembler
		assembleVerifyFunc verifyFunc
	}{
		{
			name: "rkeSecretsEncryptionCustomConfig",
			cleanupVerifyFunc: func(t *testing.T, cluster *apimgmtv3.Cluster) {
				assert.Nil(t, cluster.Spec.RancherKubernetesEngineConfig.Services.KubeAPI.SecretsEncryptionConfig.CustomConfig.Resources)
			},
			secretName: cluster.Spec.ClusterSecrets.SecretsEncryptionProvidersSecret,
			key:        SecretKey,
			expected:   `[{"resources":null,"providers":[{"aesgcm":{"keys":[{"name":"testName","secret":"testSecret"}]}}]}]`,
			assembler:  assemblers.AssembleSecretsEncryptionProvidersSecretCredential,
			assembleVerifyFunc: func(t *testing.T, cluster *apimgmtv3.Cluster) {
				assert.Equal(t, testCluster.Spec.RancherKubernetesEngineConfig.Services.KubeAPI.SecretsEncryptionConfig, cluster.Spec.RancherKubernetesEngineConfig.Services.KubeAPI.SecretsEncryptionConfig)
			},
		},
		{
			name:              "bastionHostSSHKey",
			cleanupVerifyFunc: emptyStringCondition(cluster.Spec.RancherKubernetesEngineConfig.BastionHost.SSHKey),
			secretName:        cluster.Spec.ClusterSecrets.BastionHostSSHKeySecret,
			key:               SecretKey,
			expected:          "sshKey",
			assembler:         assemblers.AssembleBastionHostSSHKeyCredential,
			assembleVerifyFunc: func(t *testing.T, cluster *apimgmtv3.Cluster) {
				assert.Equal(t, testCluster.Spec.RancherKubernetesEngineConfig.BastionHost.SSHKey, cluster.Spec.RancherKubernetesEngineConfig.BastionHost.SSHKey)
			},
		},
		{
			name: "kubeletExtraEnv",
			cleanupVerifyFunc: func(t *testing.T, cluster *apimgmtv3.Cluster) {
				assert.Len(t, cluster.Spec.RancherKubernetesEngineConfig.Services.Kubelet.ExtraEnv, 1)
				assert.Equal(t, "AWS_ACCESS_KEY_ID=keyId", cluster.Spec.RancherKubernetesEngineConfig.Services.Kubelet.ExtraEnv[0])
			},
			secretName: cluster.Spec.ClusterSecrets.KubeletExtraEnvSecret,
			key:        SecretKey,
			expected:   "secret",
			assembler:  assemblers.AssembleKubeletExtraEnvCredential,
			assembleVerifyFunc: func(t *testing.T, cluster *apimgmtv3.Cluster) {
				assert.Equal(t, testCluster.Spec.RancherKubernetesEngineConfig.Services.Kubelet.ExtraEnv, cluster.Spec.RancherKubernetesEngineConfig.Services.Kubelet.ExtraEnv)
			},
		},
		{
			name: "privateRegistryECR",
			cleanupVerifyFunc: emptyStringCondition(
				cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries[0].ECRCredentialPlugin.AwsSecretAccessKey,
				cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries[0].ECRCredentialPlugin.AwsSessionToken,
			),
			secretName: cluster.Spec.ClusterSecrets.PrivateRegistryECRSecret,
			key:        SecretKey,
			expected:   `{"testurl":"{\"awsSecretAccessKey\":\"secret\",\"awsAccessToken\":\"token\"}"}`,
			assembler:  assemblers.AssemblePrivateRegistryECRCredential,
			assembleVerifyFunc: func(t *testing.T, cluster *apimgmtv3.Cluster) {
				assert.Equal(t, testCluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries, cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cleanupVerifyFunc(t, cluster)
			assert.NotEqual(t, "", tt.secretName)
			secret, err := h.migrator.secretLister.Get(secretsNS, tt.secretName)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, string(secret.Data[tt.key]))
			cluster.Spec, err = tt.assembler(tt.secretName, "", "", cluster.Spec, h.migrator.secretLister)
			assert.Nil(t, err)
			tt.assembleVerifyFunc(t, cluster)
		})
	}

	assert.True(t, apimgmtv3.ClusterConditionRKESecretsMigrated.IsTrue(cluster))
}

func TestMigrateACISecrets(t *testing.T) {
	h := newTestHandler(t)
	defer resetMockClusters()
	defer resetMockSecrets()

	testCluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testcluster",
		},
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				RancherKubernetesEngineConfig: &rketypes.RancherKubernetesEngineConfig{
					Network: rketypes.NetworkConfig{
						Plugin: "aci",
						AciNetworkProvider: &rketypes.AciNetworkProvider{
							Token:          "secret",
							ApicUserKey:    "secret",
							KafkaClientKey: "secret",
						},
					},
				},
			},
		},
	}
	_, err := h.clusters.Create(testCluster)
	assert.Nil(t, err)
	cluster, err := h.migrateACISecrets(testCluster)
	assert.Nil(t, err)

	// test that cluster object does not get updated if migrated again
	clusterCopy := cluster.DeepCopy()
	clusterCopy, err = h.migrateACISecrets(clusterCopy)
	assert.Nil(t, err)
	assert.Equal(t, cluster, clusterCopy)

	type verifyFunc func(t *testing.T, cluster *apimgmtv3.Cluster)

	emptyStringCondition := func(fields ...string) verifyFunc {
		return func(t *testing.T, cluster *apimgmtv3.Cluster) {
			for _, field := range fields {
				assert.Equal(t, "", field)
			}
		}
	}

	tests := []struct {
		name               string
		cleanupVerifyFunc  verifyFunc
		secretName         string
		key                string
		expected           string
		assembler          assemblers.Assembler
		assembleVerifyFunc verifyFunc
	}{
		{
			name:              "token",
			cleanupVerifyFunc: emptyStringCondition(cluster.Spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.Token),
			secretName:        cluster.Spec.ClusterSecrets.ACITokenSecret,
			key:               SecretKey,
			expected:          "secret",
			assembler:         assemblers.AssembleACITokenCredential,
			assembleVerifyFunc: func(t *testing.T, cluster *apimgmtv3.Cluster) {
				assert.Equal(t, testCluster.Spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.Token, cluster.Spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.Token)
			},
		},
		{
			name:              "user key",
			cleanupVerifyFunc: emptyStringCondition(cluster.Spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.ApicUserKey),
			secretName:        cluster.Spec.ClusterSecrets.ACIAPICUserKeySecret,
			key:               SecretKey,
			expected:          "secret",
			assembler:         assemblers.AssembleACIAPICUserKeyCredential,
			assembleVerifyFunc: func(t *testing.T, cluster *apimgmtv3.Cluster) {
				assert.Equal(t, testCluster.Spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.ApicUserKey, cluster.Spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.ApicUserKey)
			},
		},
		{
			name:              "kafka key",
			cleanupVerifyFunc: emptyStringCondition(cluster.Spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.KafkaClientKey),
			secretName:        cluster.Spec.ClusterSecrets.ACIKafkaClientKeySecret,
			key:               SecretKey,
			expected:          "secret",
			assembler:         assemblers.AssembleACIKafkaClientKeyCredential,
			assembleVerifyFunc: func(t *testing.T, cluster *apimgmtv3.Cluster) {
				assert.Equal(t, testCluster.Spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.KafkaClientKey, cluster.Spec.RancherKubernetesEngineConfig.Network.AciNetworkProvider.KafkaClientKey)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cleanupVerifyFunc(t, cluster)
			assert.NotEqual(t, "", tt.secretName)
			secret, err := h.migrator.secretLister.Get(secretsNS, tt.secretName)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, string(secret.Data[tt.key]))
			cluster.Spec, err = tt.assembler(tt.secretName, "", "", cluster.Spec, h.migrator.secretLister)
			assert.Nil(t, err)
			tt.assembleVerifyFunc(t, cluster)
		})
	}

	assert.True(t, apimgmtv3.ClusterConditionACISecretsMigrated.IsTrue(cluster))
}

func TestSync(t *testing.T) {
	h := newTestHandler(t)
	defer resetMockClusters()
	defer resetMockSecrets()
	testCluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testcluster",
		},
	}
	testCluster, err := h.clusters.Create(testCluster)
	assert.Nil(t, err)
	got, err := h.sync("", testCluster)
	assert.Nil(t, err)
	assert.True(t, apimgmtv3.ClusterConditionSecretsMigrated.IsTrue(got))
	assert.True(t, apimgmtv3.ClusterConditionServiceAccountSecretsMigrated.IsTrue(got))
	assert.True(t, apimgmtv3.ClusterConditionRKESecretsMigrated.IsTrue(got))
	assert.True(t, apimgmtv3.ClusterConditionACISecretsMigrated.IsTrue(got))

	testClusterCopy := got.(*apimgmtv3.Cluster).DeepCopy()
	got, err = h.sync("", testClusterCopy)

	assert.Nil(t, err)
	assert.Equal(t, got, testClusterCopy)
}
