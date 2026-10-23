package activedirectory

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/norman/objectclient"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

func TestADProviderGetPrincipalUsesTokenConfig(t *testing.T) {
	tests := map[string]struct {
		principalID string
		wantFetched []string
		wantErr     string
	}{
		"principal from the token's config": {
			principalID: "corp-ad_user://cn=bob,dc=corp,dc=example,dc=com",
			wantFetched: []string{"corp-ad"},
		},
		"principal from another config": {
			principalID: "it-ad_user://cn=bob,dc=it,dc=example,dc=com",
			wantErr:     "not issued by",
		},
		"group principal from another config": {
			principalID: "it-ad_group://cn=admins,dc=it,dc=example,dc=com",
			wantErr:     "not issued by",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// The handler runs on the server's goroutines, so access to
			// fetched is guarded and failures are reported with assert, not
			// require (FailNow must only be called from the test goroutine).
			var (
				mu      sync.Mutex
				fetched []string
			)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				configName := path.Base(r.URL.Path)
				mu.Lock()
				fetched = append(fetched, configName)
				mu.Unlock()

				// No servers are configured, so any LDAP lookup fails after
				// the config has been loaded.
				w.Header().Set("Content-Type", "application/json")
				assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"apiVersion": "management.cattle.io/v3",
					"kind":       "AuthConfig",
					"metadata":   map[string]any{"name": configName},
					"enabled":    true,
				}))
			}))
			t.Cleanup(srv.Close)

			objectClient := newTestAuthConfigObjectClient(t, srv.URL)
			p := &adProvider{
				authConfigs: &fakes.AuthConfigInterfaceMock{
					ObjectClientFunc: func() *objectclient.ObjectClient { return objectClient },
				},
			}
			token := &v3.Token{
				UserPrincipal: v3.Principal{
					ObjectMeta: metav1.ObjectMeta{Name: "corp-ad_user://alice"},
				},
			}

			_, err := p.GetPrincipal(tt.principalID, token)
			// Lookups for the token's config fail because no LDAP server is
			// configured. The success path can't be exercised here because
			// adProvider.ldapConnection always dials a real LDAP server; it
			// is not injectable with the ldap fakes.
			require.Error(t, err)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			}
			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, tt.wantFetched, fetched)
		})
	}
}

func newTestAuthConfigObjectClient(t *testing.T, host string) *objectclient.ObjectClient {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v3.AddToScheme(scheme))
	gv := mgmtv3.AuthConfigGroupVersionKind.GroupVersion()
	restClient, err := rest.RESTClientFor(&rest.Config{
		Host:    host,
		APIPath: "/apis",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &gv,
			NegotiatedSerializer: serializer.NewCodecFactory(scheme).WithoutConversion(),
		},
	})
	require.NoError(t, err)

	gvr := gv.WithResource(mgmtv3.AuthConfigResource.Name)
	return objectclient.NewObjectClient("", client.NewClient(gvr, mgmtv3.AuthConfigGroupVersionKind.Kind, false, restClient, time.Minute),
		&mgmtv3.AuthConfigResource, mgmtv3.AuthConfigGroupVersionKind, &objectclient.UnstructuredObjectFactory{})
}
