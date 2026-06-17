package clusterregistrationtokens

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterImportHandler_ValidateAuthImage(t *testing.T) {
	ch := &ClusterImport{
		Clusters: &fakes.ClusterInterfaceMock{
			GetFunc: func(name string, opts metav1.GetOptions) (*apimgmtv3.Cluster, error) {
				return &apimgmtv3.Cluster{}, nil
			},
		},
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
