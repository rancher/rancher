package oidc

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/norman/objectclient"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

func TestSaveOIDCConfigUpdatesConfigWithSecretReferences(t *testing.T) {
	tests := map[string]struct {
		configName string
		wantPrefix string
	}{
		"default config": {
			configName: "genericoidc",
			wantPrefix: "genericoidcconfig",
		},
		"additional config": {
			configName: "test-eu-oidc",
			wantPrefix: "genericoidcconfig-test-eu-oidc",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var updated map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPut, r.Method)
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(body, &updated))

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(body)
			}))
			t.Cleanup(srv.Close)

			ctrl := gomock.NewController(t)
			secretCache := wranglerfake.NewMockCacheInterface[*corev1.Secret](ctrl)
			secretCache.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, apierrors.NewNotFound(corev1.Resource("secrets"), "")).AnyTimes()
			secrets := wranglerfake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
			secrets.EXPECT().Cache().Return(secretCache).AnyTimes()
			secrets.EXPECT().Create(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
				return s, nil
			}).Times(2)

			objectClient := newTestAuthConfigObjectClient(t, srv.URL)
			o := OpenIDCProvider{
				Name:    "genericoidc",
				Type:    "genericOIDCConfig",
				Secrets: secrets,
				AuthConfigs: &fakes.AuthConfigInterfaceMock{
					ObjectClientFunc: func() *objectclient.ObjectClient { return objectClient },
				},
				GetConfig: func(configName string) (*apiv3.OIDCConfig, error) {
					return &apiv3.OIDCConfig{
						AuthConfig: apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: configName, ResourceVersion: "1"}},
					}, nil
				},
			}

			config := &apiv3.OIDCConfig{
				AuthConfig:   apiv3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: tt.configName}},
				PrivateKey:   "test-key",
				ClientSecret: "test-secret",
			}
			require.NoError(t, o.saveOIDCConfig(config))

			require.NotNil(t, updated, "the AuthConfig should be updated")
			assert.Equal(t, common.SecretsNamespace+":"+tt.wantPrefix+"-privatekey", updated["privateKey"])
			assert.Equal(t, common.SecretsNamespace+":"+tt.wantPrefix+"-clientsecret", updated["clientSecret"])
			assert.Equal(t, tt.configName, updated["metadata"].(map[string]any)["name"])
		})
	}
}

func newTestAuthConfigObjectClient(t *testing.T, host string) *objectclient.ObjectClient {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, apiv3.AddToScheme(scheme))
	gv := v3.AuthConfigGroupVersionKind.GroupVersion()
	restClient, err := rest.RESTClientFor(&rest.Config{
		Host:    host,
		APIPath: "/apis",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &gv,
			NegotiatedSerializer: serializer.NewCodecFactory(scheme).WithoutConversion(),
		},
	})
	require.NoError(t, err)

	gvr := gv.WithResource(v3.AuthConfigResource.Name)
	return objectclient.NewObjectClient("", client.NewClient(gvr, v3.AuthConfigGroupVersionKind.Kind, false, restClient, time.Minute),
		&v3.AuthConfigResource, v3.AuthConfigGroupVersionKind, &objectclient.UnstructuredObjectFactory{})
}
