package mcmauthorizer

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

func newTestSecretIndexer(t *testing.T, secrets ...*corev1.Secret) cache.Indexer {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		SecretTokenIndex: func(obj interface{}) ([]string, error) {
			secret, ok := obj.(*corev1.Secret)
			if !ok {
				return nil, nil
			}
			return (&Authorizer{}).secretTokenIndex(secret)
		},
	})
	for _, s := range secrets {
		if err := indexer.Add(s); err != nil {
			t.Fatal(err)
		}
	}
	return indexer
}

func tokenSecret(namespace, name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Data:       data,
	}
}

func TestGetClusterByToken(t *testing.T) {
	cluster := &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-abc"}}
	clusterLister := &fakes.ClusterListerMock{
		GetFunc: func(namespace, name string) (*apimgmtv3.Cluster, error) {
			if name == "c-abc" {
				return cluster, nil
			}
			return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "clusters"}, name)
		},
	}
	// c-old is the namespace of a cluster that was deleted; its tokens are still being torn down.
	namespaceLister := &corefakes.NamespaceListerMock{
		GetFunc: func(_, name string) (*corev1.Namespace, error) {
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
			if name == "c-old" {
				ns.Status.Phase = corev1.NamespaceTerminating
			}
			return ns, nil
		},
	}

	tests := []struct {
		name       string
		secrets    []*corev1.Secret
		token      string
		wantResult *apimgmtv3.Cluster
		wantErr    error
		wantErrMsg string
	}{
		{
			name: "current token matches",
			secrets: []*corev1.Secret{
				tokenSecret("c-abc", "crt-token-system", map[string][]byte{"token": []byte("tok")}),
			},
			token:      "tok",
			wantResult: cluster,
		},
		{
			name: "previous token matches during rotation",
			secrets: []*corev1.Secret{
				tokenSecret("c-abc", "crt-token-system", map[string][]byte{
					"token":         []byte("new-tok"),
					"previousToken": []byte("old-tok"),
				}),
			},
			token:      "old-tok",
			wantResult: cluster,
		},
		{
			name: "non-token secret with matching data is ignored",
			secrets: []*corev1.Secret{
				tokenSecret("c-abc", "unrelated-secret", map[string][]byte{"token": []byte("tok")}),
			},
			token:   "tok",
			wantErr: ErrClusterNotFound,
		},
		{
			name: "secret namespace has no matching cluster",
			secrets: []*corev1.Secret{
				tokenSecret("c-missing", "crt-token-system", map[string][]byte{"token": []byte("tok")}),
			},
			token:   "tok",
			wantErr: ErrClusterNotFound,
		},
		{
			name:    "no matching secret",
			token:   "tok",
			wantErr: ErrClusterNotFound,
		},
		{
			name: "token from a previous cluster with the same name",
			secrets: []*corev1.Secret{
				tokenSecret("c-old", "crt-token-system", map[string][]byte{"token": []byte("tok")}),
			},
			token:      "tok",
			wantErr:    ErrClusterNotFound,
			wantErrMsg: "registration token belongs to a previous cluster named c-old",
		},
		{
			name: "a stale match does not hide a usable one",
			secrets: []*corev1.Secret{
				tokenSecret("c-old", "crt-token-system", map[string][]byte{"token": []byte("tok")}),
				tokenSecret("c-abc", "crt-token-system", map[string][]byte{"token": []byte("tok")}),
			},
			token:      "tok",
			wantResult: cluster,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &Authorizer{
				secretIndexer:   newTestSecretIndexer(t, tt.secrets...),
				clusterLister:   clusterLister,
				namespaceLister: namespaceLister,
			}
			got, err := auth.getClusterByToken(tt.token)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				if tt.wantErrMsg != "" {
					assert.ErrorContains(t, err, tt.wantErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantResult, got)
		})
	}
}

func TestFormatAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected string
	}{
		{
			name:     "bare IPv6 with port",
			address:  "2001:cafe:43::1:443",
			expected: "[2001:cafe:43::1]:443",
		},
		{
			name:     "bare IPv6 loopback with port",
			address:  "::1:6443",
			expected: "[::1]:6443",
		},
		{
			name:     "already bracketed IPv6",
			address:  "[2001:cafe:43::1]:443",
			expected: "[2001:cafe:43::1]:443",
		},
		{
			name:     "IPv4 with port",
			address:  "192.168.1.1:6443",
			expected: "192.168.1.1:6443",
		},
		{
			name:     "hostname with port",
			address:  "my-cluster.example.com:6443",
			expected: "my-cluster.example.com:6443",
		},
		{
			name:     "no port",
			address:  "192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "full IPv6 with port",
			address:  "2001:0db8:85a3:0000:0000:8a2e:0370:7334:443",
			expected: "[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:443",
		},
		{
			name:     "empty string",
			address:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAddress(tt.address)
			if result != tt.expected {
				t.Errorf("formatAddress(%q) = %q, want %q", tt.address, result, tt.expected)
			}
		})
	}
}

func TestAuthorizeTunnelTracksTheSessionsCluster(t *testing.T) {
	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-abc", UID: "uid-abc"},
		// A driver that doesn't take its connection details from the agent, so authorizing doesn't
		// update the cluster.
		Status: apimgmtv3.ClusterStatus{Driver: apimgmtv3.ClusterDriverAKS},
	}
	auth := &Authorizer{
		secretIndexer: newTestSecretIndexer(t, tokenSecret("c-abc", "crt-token-system", map[string][]byte{"token": []byte("tok")})),
		clusterLister: &fakes.ClusterListerMock{
			GetFunc: func(_, name string) (*apimgmtv3.Cluster, error) {
				if name == "c-abc" {
					return cluster, nil
				}
				return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "clusters"}, name)
			},
		},
		namespaceLister: &corefakes.NamespaceListerMock{
			GetFunc: func(_, name string) (*corev1.Namespace, error) {
				return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}, nil
			},
		},
	}
	params := base64.StdEncoding.EncodeToString([]byte(`{"cluster":{"address":"10.0.0.1:6443","token":"sa-token","caCert":"ca"}}`))

	tests := []struct {
		name        string
		token       string
		wantKey     string
		wantOK      bool
		wantTracked types.UID
	}{
		{name: "authorized", token: "tok", wantKey: "c-abc", wantOK: true, wantTracked: "uid-abc"},
		{name: "rejected", token: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v3/connect/register", nil)
			req.Header.Set(Token, tt.token)
			req.Header.Set(Params, params)

			var (
				key        string
				ok         bool
				trackedUID types.UID
			)
			tunnelserver.NewSessionTracker().Handler(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				key, ok, _ = auth.AuthorizeTunnel(r)
				_, trackedUID = tunnelserver.SessionCluster(r)
			})).ServeHTTP(httptest.NewRecorder(), req)

			assert.Equal(t, tt.wantKey, key)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantTracked, trackedUID)
		})
	}
}
