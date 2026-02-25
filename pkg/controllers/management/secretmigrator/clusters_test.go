package secretmigrator

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	v3fakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	ctrl := gomock.NewController(t)

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

	projectLister := &v3fakes.ProjectListerMock{
		ListFunc: func(namespace string, selector labels.Selector) ([]*apimgmtv3.Project, error) {
			var list []*apimgmtv3.Project
			return list, nil
		},
	}

	// Create wrangler mock for ClusterClient
	clusterClient := fake.NewMockNonNamespacedClientInterface[*apimgmtv3.Cluster, *apimgmtv3.ClusterList](ctrl)

	clusterClient.EXPECT().Create(gomock.Any()).DoAndReturn(
		func(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
			mockClusters[cluster.Name] = cluster.DeepCopy()
			mockClusters[cluster.Name].ObjectMeta.ResourceVersion = "0"
			return mockClusters[cluster.Name], nil
		},
	).AnyTimes()

	clusterClient.EXPECT().Update(gomock.Any()).DoAndReturn(
		func(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
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
	).AnyTimes()

	clusterClient.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(
		func(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
			if _, ok := mockClusters[cluster.Name]; !ok {
				return nil, apierror.NewNotFound(schema.GroupResource{}, fmt.Sprintf("cluster [%s]", cluster.Name))
			}
			// UpdateStatus should only update status fields and increment resource version
			mockClusters[cluster.Name].Status = cluster.Status
			rv, _ := strconv.Atoi(mockClusters[cluster.Name].ObjectMeta.ResourceVersion)
			rv++
			mockClusters[cluster.Name].ObjectMeta.ResourceVersion = strconv.Itoa(rv)
			return mockClusters[cluster.Name].DeepCopy(), nil
		},
	).AnyTimes()

	clusterClient.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
		func(name string, opts metav1.GetOptions) (*apimgmtv3.Cluster, error) {
			cluster, ok := mockClusters[name]
			if !ok {
				return nil, apierror.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "clusters"}, name)
			}
			return cluster.DeepCopy(), nil
		},
	).AnyTimes()

	return &handler{
		clusters:      clusterClient,
		migrator:      NewMigrator(&secretLister, &secrets),
		projectLister: projectLister,
	}
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
	assert.True(t, apimgmtv3.ClusterConditionServiceAccountSecretsMigrated.IsTrue(got))
	testClusterCopy := got.DeepCopy()
	got, err = h.sync("", testClusterCopy)

	assert.Nil(t, err)
	assert.Equal(t, got, testClusterCopy)
}
