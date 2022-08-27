package secretmigrator

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	v3fakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	rketypes "github.com/rancher/rke/types"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

var (
	mockSecrets  = make(map[string]*corev1.Secret)
	mockClusters = make(map[string]*apimgmtv3.Cluster)
)

const (
	credKey   = "credential"
	secretsNS = "cattle-global-data"
)

func resetMockSecrets() {
	mockSecrets = make(map[string]*corev1.Secret)
}

func resetMockClusters() {
	mockClusters = make(map[string]*apimgmtv3.Cluster)
}

func newTestHandler() *handler {
	secrets := corefakes.SecretInterfaceMock{
		CreateFunc: func(secret *corev1.Secret) (*corev1.Secret, error) {
			if secret.Name == "" {
				uniqueIdentifier := md5.Sum([]byte(time.Now().String()))
				secret.Name = hex.EncodeToString(uniqueIdentifier[:])
			}
			mockSecrets[fmt.Sprintf("%s:%s", secret.Namespace, secret.Name)] = secret
			return secret, nil
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
				return cluster, nil
			},
			UpdateFunc: func(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
				if _, ok := mockClusters[cluster.Name]; !ok {
					return nil, apierror.NewNotFound(schema.GroupResource{}, fmt.Sprintf("cluster [%s]", cluster.Name))
				}
				mockClusters[cluster.Name] = cluster.DeepCopy()
				return cluster, nil
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
	h := newTestHandler()
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
	_, err := h.clusters.Create(testCluster)
	assert.Nil(t, err)
	cluster, err := h.migrateClusterSecrets(testCluster)
	assert.Nil(t, err)

	assert.Equal(t, "", cluster.Spec.RancherKubernetesEngineConfig.PrivateRegistries[0].Password)
	secretName := cluster.Spec.ClusterSecrets.PrivateRegistrySecret
	assert.NotEqual(t, "", secretName)
	secret, err := h.migrator.secretLister.Get(secretsNS, secretName)
	assert.Nil(t, err)
	// this will fail
	registry := credentialprovider.DockerConfigJSON{
		Auths: credentialprovider.DockerConfig{
			"testurl": credentialprovider.DockerConfigEntry{
				Password: secretKey,
			},
		},
	}
	registryJSON, err := json.Marshal(registry)
	assert.Nil(t, err)
	registryData := map[string][]byte{
		corev1.DockerConfigJsonKey: registryJSON,
	}
	assert.Equal(t, registryData, secret.Data)

	assert.Equal(t, "", cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey)
	secretName = cluster.Spec.ClusterSecrets.S3CredentialSecret
	assert.NotEqual(t, "", secretName)
	secret, err = h.migrator.secretLister.Get(secretsNS, secretName)
	assert.Nil(t, err)
	assert.Equal(t, secretKey, secret.StringData[credKey])

	assert.Equal(t, "", cluster.Spec.RancherKubernetesEngineConfig.Network.WeaveNetworkProvider.Password)
	secretName = cluster.Spec.ClusterSecrets.WeavePasswordSecret
	assert.NotEqual(t, "", secretName)
	secret, err = h.migrator.secretLister.Get(secretsNS, secretName)
	assert.Nil(t, err)
	assert.Equal(t, secretKey, secret.StringData[credKey])

	assert.Equal(t, "", cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.Global.Password)
	secretName = cluster.Spec.ClusterSecrets.VsphereSecret
	assert.NotEqual(t, "", secretName)
	secret, err = h.migrator.secretLister.Get(secretsNS, secretName)
	assert.Nil(t, err)
	assert.Equal(t, secretKey, secret.StringData[credKey])

	assert.Equal(t, "", cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.VsphereCloudProvider.VirtualCenter["vc1"].Password)
	secretName = cluster.Spec.ClusterSecrets.VirtualCenterSecret
	assert.NotEqual(t, "", secretName)
	secret, err = h.migrator.secretLister.Get(secretsNS, secretName)
	assert.Nil(t, err)
	assert.Equal(t, secretKey, secret.StringData["vc1"])

	assert.Equal(t, "", cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.OpenstackCloudProvider.Global.Password)
	secretName = cluster.Spec.ClusterSecrets.OpenStackSecret
	assert.NotEqual(t, "", secretName)
	secret, err = h.migrator.secretLister.Get(secretsNS, secretName)
	assert.Nil(t, err)
	assert.Equal(t, secretKey, secret.StringData[credKey])

	assert.Equal(t, "", cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientSecret)
	secretName = cluster.Spec.ClusterSecrets.AADClientSecret
	assert.NotEqual(t, "", secretName)
	secret, err = h.migrator.secretLister.Get(secretsNS, secretName)
	assert.Nil(t, err)
	assert.Equal(t, secretKey, secret.StringData[credKey])

	assert.Equal(t, "", cluster.Spec.RancherKubernetesEngineConfig.CloudProvider.AzureCloudProvider.AADClientCertPassword)
	secretName = cluster.Spec.ClusterSecrets.AADClientCertSecret
	assert.NotEqual(t, "", secretName)
	secret, err = h.migrator.secretLister.Get(secretsNS, secretName)
	assert.Nil(t, err)
	assert.Equal(t, secretKey, secret.StringData[credKey])

	assert.True(t, apimgmtv3.ClusterConditionSecretsMigrated.IsTrue(cluster))

	// test that cluster object has not been modified since last update with client
	clusterFromClient, err := h.clusters.Get(cluster.Name, metav1.GetOptions{})
	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(cluster, clusterFromClient))

	testCluster2 := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testcluster2",
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
				},
			},
		},
	}
	cluster, err = h.migrateClusterSecrets(testCluster2)
	assert.Equal(t, err.Error(), fmt.Sprintf(" \"cluster [%s]\" not found", testCluster2.Name))
	// no change should
	assert.Equal(t, cluster, testCluster2)
	assert.True(t, apimgmtv3.ClusterConditionSecretsMigrated.IsFalse(cluster))
}

func TestMigrateClusterServiceAccountToken(t *testing.T) {
	h := newTestHandler()
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
	assert.Equal(t, secret.StringData[credKey], token)
	assert.True(t, apimgmtv3.ClusterConditionServiceAccountSecretsMigrated.IsTrue(cluster))

	// test that cluster object has not been modified since last update with client
	clusterFromClient, err := h.clusters.Get(cluster.Name, metav1.GetOptions{})
	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(cluster, clusterFromClient))
}
