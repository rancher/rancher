package clusterregistrationtokens

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/tunnelserver/mcmauthorizer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// newTestNamespaceLister returns a namespace lister whose namespaces are all active, or all terminating.
func newTestNamespaceLister(terminating bool) *corefakes.NamespaceListerMock {
	return &corefakes.NamespaceListerMock{
		GetFunc: func(_, name string) (*corev1.Namespace, error) {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
			if terminating {
				now := metav1.Now()
				ns.DeletionTimestamp = &now
				ns.Status.Phase = corev1.NamespaceTerminating
			}
			return ns, nil
		},
	}
}

func TestIsValidTokenRejectsATokenFromAPreviousCluster(t *testing.T) {
	ch := &ClusterImport{
		SecretIndexer:   newTestSecretIndexer("cluster", "token"),
		NamespaceLister: newTestNamespaceLister(true),
	}

	assert.False(t, ch.isValidToken(newTestCluster("cluster", metav1.Time{}), "token"))
}

func newTestCluster(name string, created metav1.Time) *apimgmtv3.Cluster {
	return &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: created}}
}

func TestIsValidTokenAcceptsATokenOfTheCurrentCluster(t *testing.T) {
	ch := &ClusterImport{
		SecretIndexer:   newTestSecretIndexer("cluster", "token"),
		NamespaceLister: newTestNamespaceLister(false),
	}

	assert.True(t, ch.isValidToken(newTestCluster("cluster", metav1.Time{}), "token"))
	assert.False(t, ch.isValidToken(newTestCluster("other-cluster", metav1.Time{}), "token"), "a token only belongs to the cluster named by its namespace")
}

func newTestSecretIndexer(clusterID, token string) cache.Indexer {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		mcmauthorizer.SecretTokenIndex: func(obj interface{}) ([]string, error) {
			return []string{token}, nil
		},
	})
	_ = indexer.Add(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "crt-token-system", Namespace: clusterID},
	})
	return indexer
}

func TestClusterImportHandler_ValidateAuthImage(t *testing.T) {
	ch := &ClusterImport{
		Clusters: &fakes.ClusterInterfaceMock{
			GetFunc: func(name string, opts metav1.GetOptions) (*apimgmtv3.Cluster, error) {
				return &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name}}, nil
			},
		},
		SecretIndexer:   newTestSecretIndexer("cluster", "token"),
		NamespaceLister: newTestNamespaceLister(false),
	}

	tests := []struct {
		name      string
		authImage string
		wantCode  int
	}{
		{"fully qualified image", "rancher/kube-api-auth:v0.2.6", http.StatusOK},
		{"image with registry", "registry.example.com/rancher/kube-api-auth:v0.2.6", http.StatusOK},
		{"image with digest", "rancher/kube-api-auth@sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", http.StatusOK},
		{"empty image", "", http.StatusOK},
		{"contains newline", "myimage:latest%0A%20%20command:%20bad", http.StatusBadRequest},
		{"contains space", "myimage:latest%20extra", http.StatusBadRequest},
		{"contains multiline content", "myimage:latest%0A%20%20command:%20%5B%22sh%22%5D", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/v3/import/token_cluster.yaml"
			if tt.authImage != "" {
				url += "?authImage=" + tt.authImage
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.SetPathValue("filename", "token_cluster.yaml")
			resp := httptest.NewRecorder()

			ch.ClusterImportHandler(resp, req)

			assert.Equal(t, tt.wantCode, resp.Code)
			if tt.wantCode == http.StatusBadRequest {
				assert.True(t, strings.HasPrefix(resp.Body.String(), "invalid authImage - "))
			}
		})
	}
}
