package publicapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestV1AuthProviderStoreList(t *testing.T) {
	ctrl := gomock.NewController(t)
	authConfigCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.AuthConfig](ctrl)
	authConfigCache.EXPECT().List(labels.Everything()).Return([]*apiv3.AuthConfig{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "local"},
			Type:       "localConfig",
			Enabled:    true,
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "okta"},
			Type:       "oktaConfig",
			Enabled:    true,
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "github"},
			Type:       "githubConfig",
		},
	}, nil).Times(1)

	localConfig := &unstructured.Unstructured{}
	localConfig.SetUnstructuredContent(map[string]any{
		"apiVersion": "management.cattle.io/v3",
		"kind":       "AuthConfig",
		"metadata": map[string]any{
			"name": "local",
		},
		"type":    "localConfig",
		"enabled": true,
	})
	oktaConfig := &unstructured.Unstructured{}
	oktaConfig.SetUnstructuredContent(map[string]any{
		"apiVersion": "management.cattle.io/v3",
		"kind":       "AuthConfig",
		"metadata": map[string]any{
			"name": "okta",
		},
		"type":    "oktaConfig",
		"enabled": true,
	})
	githubConfig := &unstructured.Unstructured{}
	githubConfig.SetUnstructuredContent(map[string]any{
		"apiVersion": "management.cattle.io/v3",
		"kind":       "AuthConfig",
		"metadata": map[string]any{
			"name": "github",
		},
		"type": "githubConfig",
	})

	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), localConfig, oktaConfig, githubConfig)
	authConfigUnstructured := fakeDynamicClient.Resource(schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3",
		Resource: "authconfigs",
	})

	getProviderByType := func(providerType string) common.AuthProvider {
		switch providerType {
		case "localConfig":
			return &local.Provider{}
		case "oktaConfig":
			return &saml.Provider{}
		case "githubConfig":
			return &github.Provider{}
		default:
			require.FailNow(t, "unexpected provider type "+providerType)
			return nil
		}
	}

	store := v1AuthProviderStore{
		authConfigCache:         authConfigCache,
		authConfigsUnstructured: authConfigUnstructured,
		getProviderByType:       getProviderByType,
	}

	r := httptest.NewRequest("GET", "/v1-public/authproviders", nil)
	w := httptest.NewRecorder()

	store.List(w, r)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
	gotPayload := map[string]any{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &gotPayload))
	wantPayload := map[string]any{
		"data": []any{
			map[string]any{
				"id":                 "local",
				"type":               "localProvider",
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
				"logoutAllSupported": false,
			},
			map[string]any{
				"id":                 "okta",
				"type":               "oktaProvider",
				"logoutAllEnabled":   false,
				"logoutAllForced":    false,
				"logoutAllSupported": false,
			},
		},
	}
	require.Equal(t, wantPayload, gotPayload)
}

func TestV1AuthTokenStoreGet(t *testing.T) {
	tokenID := "token-5lwps"
	bearerToken := tokenID + ":jslbp8qbkvpndjj4xmvl9crwh7w96pvxrg4xltsmcbcvvcrk9thphq"

	authToken := &apiv3.SamlToken{
		ObjectMeta: metav1.ObjectMeta{Name: tokenID},
		Token:      bearerToken,
		ExpiresAt:  time.Now().Add(1 * time.Hour).Format(time.RFC3339),
	}

	ctrl := gomock.NewController(t)
	tokens := fake.NewMockClientInterface[*apiv3.SamlToken, *apiv3.SamlTokenList](ctrl)
	tokens.EXPECT().Get(namespace.GlobalNamespace, tokenID, gomock.Any()).Return(authToken, nil).Times(1)

	store := v1AuthTokenStore{
		tokens: tokens,
	}

	r := httptest.NewRequest(http.MethodGet, "/v1-public/authtokens/"+tokenID, nil)
	r = mux.SetURLVars(r, map[string]string{"id": tokenID})
	w := httptest.NewRecorder()

	store.Get(w, r)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
	gotPayload := map[string]any{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &gotPayload))
	wantPayload := map[string]any{
		"token":     authToken.Token,
		"expiresAt": authToken.ExpiresAt,
	}
	require.Equal(t, wantPayload, gotPayload)
}

func TestV1AuthTokenStoreDelete(t *testing.T) {
	tokenID := "token-5lwps"

	ctrl := gomock.NewController(t)
	tokens := fake.NewMockClientInterface[*apiv3.SamlToken, *apiv3.SamlTokenList](ctrl)
	tokens.EXPECT().Delete(namespace.GlobalNamespace, tokenID, gomock.Any()).Return(nil).Times(1)

	store := v1AuthTokenStore{
		tokens: tokens,
	}

	r := httptest.NewRequest(http.MethodDelete, "/v1-public/authtokens/"+tokenID, nil)
	r = mux.SetURLVars(r, map[string]string{"id": tokenID})
	w := httptest.NewRecorder()

	store.Delete(w, r)
	require.Equal(t, 200, w.Result().StatusCode)
}
