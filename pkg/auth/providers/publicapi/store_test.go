package publicapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/objectclient"
	normantypes "github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/mocks"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
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

func TestV1AuthProviderStoreListLocalHidden(t *testing.T) {
	ctrl := gomock.NewController(t)

	// externalMock stands in for the global provider registry entry that makes
	// IsExternalProviderEnabled() return true, which in turn makes IsLocalHidden() true.
	externalMock := mocks.NewMockAuthProvider(ctrl)
	externalMock.EXPECT().IsDisabledProvider().Return(false, nil).AnyTimes()

	providers.SetProviders(map[string]common.AuthProvider{"okta": externalMock})
	defer providers.SetProviders(nil)
	features.HideLocalAuthProvider.Set(true)
	defer features.HideLocalAuthProvider.Set(false)

	authConfigCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.AuthConfig](ctrl)
	authConfigCache.EXPECT().List(labels.Everything()).Return([]*apiv3.AuthConfig{
		{ObjectMeta: metav1.ObjectMeta{Name: local.Name}, Type: "localConfig", Enabled: true},
		{ObjectMeta: metav1.ObjectMeta{Name: "okta"}, Type: "oktaConfig", Enabled: true},
	}, nil).Times(1)

	oktaConfig := &unstructured.Unstructured{}
	oktaConfig.SetUnstructuredContent(map[string]any{
		"apiVersion": "management.cattle.io/v3",
		"kind":       "AuthConfig",
		"metadata":   map[string]any{"name": "okta"},
		"type":       "oktaConfig",
		"enabled":    true,
	})

	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), oktaConfig)
	authConfigUnstructured := fakeDynamicClient.Resource(schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3",
		Resource: "authconfigs",
	})

	store := v1AuthProviderStore{
		authConfigCache:         authConfigCache,
		authConfigsUnstructured: authConfigUnstructured,
		getProviderByType: func(providerType string) common.AuthProvider {
			if providerType == "oktaConfig" {
				return &saml.Provider{}
			}
			require.FailNow(t, "unexpected provider type "+providerType)
			return nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/v1-public/authproviders", nil)
	w := httptest.NewRecorder()

	store.List(w, r)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	gotPayload := map[string]any{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &gotPayload))
	data, ok := gotPayload["data"].([]any)
	require.True(t, ok)
	require.Len(t, data, 1)
	require.Equal(t, "okta", data[0].(map[string]any)["id"])
}

func TestV1AuthProviderStoreListUnregisteredType(t *testing.T) {
	ctrl := gomock.NewController(t)

	authConfigCache := fake.NewMockNonNamespacedCacheInterface[*apiv3.AuthConfig](ctrl)
	authConfigCache.EXPECT().List(labels.Everything()).Return([]*apiv3.AuthConfig{
		{ObjectMeta: metav1.ObjectMeta{Name: "unknown"}, Type: "unknownConfig", Enabled: true},
		{ObjectMeta: metav1.ObjectMeta{Name: "okta"}, Type: "oktaConfig", Enabled: true},
	}, nil).Times(1)

	unknownConfig := &unstructured.Unstructured{}
	unknownConfig.SetUnstructuredContent(map[string]any{
		"apiVersion": "management.cattle.io/v3",
		"kind":       "AuthConfig",
		"metadata":   map[string]any{"name": "unknown"},
		"type":       "unknownConfig",
		"enabled":    true,
	})
	oktaConfig := &unstructured.Unstructured{}
	oktaConfig.SetUnstructuredContent(map[string]any{
		"apiVersion": "management.cattle.io/v3",
		"kind":       "AuthConfig",
		"metadata":   map[string]any{"name": "okta"},
		"type":       "oktaConfig",
		"enabled":    true,
	})

	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), unknownConfig, oktaConfig)
	authConfigUnstructured := fakeDynamicClient.Resource(schema.GroupVersionResource{
		Group:    "management.cattle.io",
		Version:  "v3",
		Resource: "authconfigs",
	})

	store := v1AuthProviderStore{
		authConfigCache:         authConfigCache,
		authConfigsUnstructured: authConfigUnstructured,
		getProviderByType: func(providerType string) common.AuthProvider {
			if providerType == "oktaConfig" {
				return &saml.Provider{}
			}
			return nil
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/v1-public/authproviders", nil)
	w := httptest.NewRecorder()

	store.List(w, r)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)
	gotPayload := map[string]any{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &gotPayload))
	data, ok := gotPayload["data"].([]any)
	require.True(t, ok)
	require.Len(t, data, 1)
	require.Equal(t, "okta", data[0].(map[string]any)["id"])
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
	r.SetPathValue("id", tokenID)
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
	r.SetPathValue("id", tokenID)
	w := httptest.NewRecorder()

	store.Delete(w, r)
	require.Equal(t, 200, w.Result().StatusCode)
}

func TestAuthProvidersStoreByID(t *testing.T) {
	ctrl := gomock.NewController(t)

	localMock := mocks.NewMockAuthProvider(ctrl)
	localMock.EXPECT().TransformToAuthProvider(gomock.Any()).
		Return(map[string]any{"id": "local", "type": "localProvider"}, nil).AnyTimes()

	externalMock := mocks.NewMockAuthProvider(ctrl)
	externalMock.EXPECT().IsDisabledProvider().Return(false, nil).AnyTimes()
	externalMock.EXPECT().TransformToAuthProvider(gomock.Any()).
		Return(map[string]any{"id": "github", "type": "githubProvider"}, nil).AnyTimes()

	makeUnstructured := func(obj map[string]any) runtime.Object {
		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(obj)
		return u
	}

	tests := []struct {
		name     string
		id       string
		flag     bool
		registry map[string]common.AuthProvider
		getFunc  func(string, metav1.GetOptions) (runtime.Object, error)
		wantErr  bool
		want404  bool
		wantData map[string]any
	}{
		{
			name: "local provider returns 404 when hidden",
			id:   local.Name,
			flag: true,
			registry: map[string]common.AuthProvider{
				local.Name: localMock,
				"github":   externalMock,
			},
			getFunc: func(_ string, _ metav1.GetOptions) (runtime.Object, error) {
				return makeUnstructured(map[string]any{"type": "localConfig"}), nil
			},
			wantErr: true,
			want404: true,
		},
		{
			name: "local provider returned when feature flag is off",
			id:   local.Name,
			flag: false,
			registry: map[string]common.AuthProvider{
				local.Name: localMock,
			},
			getFunc: func(_ string, _ metav1.GetOptions) (runtime.Object, error) {
				return makeUnstructured(map[string]any{"type": "localConfig"}), nil
			},
			wantData: map[string]any{"id": "local", "type": "localProvider"},
		},
		{
			name:     "Kubernetes error propagates",
			id:       "github",
			flag:     false,
			registry: map[string]common.AuthProvider{},
			getFunc: func(_ string, _ metav1.GetOptions) (runtime.Object, error) {
				return nil, fmt.Errorf("api server unavailable")
			},
			wantErr: true,
		},
		{
			name:     "unregistered provider type returns 404",
			id:       "unknown",
			flag:     false,
			registry: map[string]common.AuthProvider{},
			getFunc: func(_ string, _ metav1.GetOptions) (runtime.Object, error) {
				return makeUnstructured(map[string]any{"type": "unknownConfig"}), nil
			},
			wantErr: true,
			want404: true,
		},
		{
			name: "registered provider returns transformed data",
			id:   "github",
			flag: false,
			registry: map[string]common.AuthProvider{
				"github": externalMock,
			},
			getFunc: func(_ string, _ metav1.GetOptions) (runtime.Object, error) {
				return makeUnstructured(map[string]any{"type": "githubConfig"}), nil
			},
			wantData: map[string]any{"id": "github", "type": "githubProvider"},
		},
		{
			name:     "config missing type field returns 404",
			id:       "github",
			flag:     false,
			registry: map[string]common.AuthProvider{},
			getFunc: func(_ string, _ metav1.GetOptions) (runtime.Object, error) {
				return makeUnstructured(map[string]any{"name": "github"}), nil
			},
			wantErr: true,
			want404: true,
		},
	}

	apiCtx := &normantypes.APIContext{
		Request: httptest.NewRequest(http.MethodGet, "/v3-public/authproviders/local", nil),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features.HideLocalAuthProvider.Set(tt.flag)
			defer features.HideLocalAuthProvider.Set(false)
			providers.SetProviders(tt.registry)
			defer providers.SetProviders(nil)

			store := &authProvidersStore{
				authConfigsRaw: &fakeAuthConfigsRaw{get: tt.getFunc},
			}

			got, err := store.ByID(apiCtx, nil, tt.id)
			if tt.wantErr {
				require.Error(t, err)
				if tt.want404 {
					var apiErr *httperror.APIError
					require.ErrorAs(t, err, &apiErr)
					require.Equal(t, httperror.NotFound, apiErr.Code)
				}
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantData, got)
		})
	}
}

func TestAuthProvidersStoreList(t *testing.T) {
	ctrl := gomock.NewController(t)

	localMock := mocks.NewMockAuthProvider(ctrl)
	localMock.EXPECT().GetName().Return(local.Name).AnyTimes()
	localMock.EXPECT().TransformToAuthProvider(gomock.Any()).
		Return(map[string]any{"id": "local", "type": "localProvider"}, nil).AnyTimes()

	externalMock := mocks.NewMockAuthProvider(ctrl)
	externalMock.EXPECT().GetName().Return("github").AnyTimes()
	externalMock.EXPECT().IsDisabledProvider().Return(false, nil).AnyTimes()
	externalMock.EXPECT().TransformToAuthProvider(gomock.Any()).
		Return(map[string]any{"id": "github", "type": "githubProvider"}, nil).AnyTimes()

	makeList := func(objs ...map[string]any) *unstructured.UnstructuredList {
		l := &unstructured.UnstructuredList{}
		for _, obj := range objs {
			item := unstructured.Unstructured{Object: obj}
			l.Items = append(l.Items, *item.DeepCopy())
		}
		return l
	}

	tests := []struct {
		name     string
		flag     bool
		registry map[string]common.AuthProvider
		listFunc func(metav1.ListOptions) (runtime.Object, error)
		want     []map[string]any
	}{
		{
			name: "local excluded when hidden",
			flag: true,
			registry: map[string]common.AuthProvider{
				local.Name: localMock,
				"github":   externalMock,
			},
			listFunc: func(_ metav1.ListOptions) (runtime.Object, error) {
				return makeList(
					map[string]any{"type": "localConfig", "enabled": true},
					map[string]any{"type": "githubConfig", "enabled": true},
				), nil
			},
			want: []map[string]any{{"id": "github", "type": "githubProvider"}},
		},
		{
			name: "local included when feature flag is off",
			flag: false,
			registry: map[string]common.AuthProvider{
				local.Name: localMock,
				"github":   externalMock,
			},
			listFunc: func(_ metav1.ListOptions) (runtime.Object, error) {
				return makeList(
					map[string]any{"type": "localConfig", "enabled": true},
					map[string]any{"type": "githubConfig", "enabled": true},
				), nil
			},
			want: []map[string]any{
				{"id": "local", "type": "localProvider"},
				{"id": "github", "type": "githubProvider"},
			},
		},
		{
			name: "item with unregistered provider type skipped",
			flag: false,
			registry: map[string]common.AuthProvider{
				"github": externalMock,
			},
			listFunc: func(_ metav1.ListOptions) (runtime.Object, error) {
				return makeList(
					map[string]any{"type": "unknownConfig", "enabled": true},
					map[string]any{"type": "githubConfig", "enabled": true},
				), nil
			},
			want: []map[string]any{{"id": "github", "type": "githubProvider"}},
		},
		{
			name: "disabled items skipped",
			flag: false,
			registry: map[string]common.AuthProvider{
				"github": externalMock,
			},
			listFunc: func(_ metav1.ListOptions) (runtime.Object, error) {
				return makeList(
					map[string]any{"type": "githubConfig", "enabled": false},
					map[string]any{"type": "githubConfig", "enabled": true},
				), nil
			},
			want: []map[string]any{{"id": "github", "type": "githubProvider"}},
		},
		{
			name: "items missing type field skipped",
			flag: false,
			registry: map[string]common.AuthProvider{
				"github": externalMock,
			},
			listFunc: func(_ metav1.ListOptions) (runtime.Object, error) {
				return makeList(
					map[string]any{"name": "notype", "enabled": true},
					map[string]any{"type": "githubConfig", "enabled": true},
				), nil
			},
			want: []map[string]any{{"id": "github", "type": "githubProvider"}},
		},
		{
			name:     "empty list returns nil",
			flag:     false,
			registry: map[string]common.AuthProvider{},
			listFunc: func(_ metav1.ListOptions) (runtime.Object, error) {
				return makeList(), nil
			},
			want: nil,
		},
	}

	apiCtx := &normantypes.APIContext{
		Request: httptest.NewRequest(http.MethodGet, "/v3-public/authproviders", nil),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features.HideLocalAuthProvider.Set(tt.flag)
			defer features.HideLocalAuthProvider.Set(false)
			providers.SetProviders(tt.registry)
			defer providers.SetProviders(nil)

			store := &authProvidersStore{
				authConfigsRaw: &fakeAuthConfigsRaw{list: tt.listFunc},
			}

			got, err := store.List(apiCtx, nil, nil)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// fakeAuthConfigsRaw implements objectclient.GenericClient for tests.
// Only Get and List are wired up; all other methods panic if called.
type fakeAuthConfigsRaw struct {
	get  func(string, metav1.GetOptions) (runtime.Object, error)
	list func(metav1.ListOptions) (runtime.Object, error)
}

func (f *fakeAuthConfigsRaw) Get(name string, opts metav1.GetOptions) (runtime.Object, error) {
	if f.get == nil {
		panic("unexpected call to Get")
	}
	return f.get(name, opts)
}

func (f *fakeAuthConfigsRaw) List(opts metav1.ListOptions) (runtime.Object, error) {
	if f.list == nil {
		panic("unexpected call to List")
	}
	return f.list(opts)
}

func (f *fakeAuthConfigsRaw) UnstructuredClient() objectclient.GenericClient { panic("not called") }
func (f *fakeAuthConfigsRaw) GroupVersionKind() schema.GroupVersionKind      { panic("not called") }
func (f *fakeAuthConfigsRaw) Create(_ runtime.Object) (runtime.Object, error) {
	panic("not called")
}
func (f *fakeAuthConfigsRaw) GetNamespaced(_, _ string, _ metav1.GetOptions) (runtime.Object, error) {
	panic("not called")
}
func (f *fakeAuthConfigsRaw) Update(_ string, _ runtime.Object) (runtime.Object, error) {
	panic("not called")
}
func (f *fakeAuthConfigsRaw) UpdateStatus(_ string, _ runtime.Object) (runtime.Object, error) {
	panic("not called")
}
func (f *fakeAuthConfigsRaw) DeleteNamespaced(_, _ string, _ *metav1.DeleteOptions) error {
	panic("not called")
}
func (f *fakeAuthConfigsRaw) Delete(_ string, _ *metav1.DeleteOptions) error { panic("not called") }
func (f *fakeAuthConfigsRaw) ListNamespaced(_ string, _ metav1.ListOptions) (runtime.Object, error) {
	panic("not called")
}
func (f *fakeAuthConfigsRaw) Watch(_ metav1.ListOptions) (watch.Interface, error) {
	panic("not called")
}
func (f *fakeAuthConfigsRaw) DeleteCollection(_ *metav1.DeleteOptions, _ metav1.ListOptions) error {
	panic("not called")
}
func (f *fakeAuthConfigsRaw) Patch(_ string, _ runtime.Object, _ k8stypes.PatchType, _ []byte, _ ...string) (runtime.Object, error) {
	panic("not called")
}
func (f *fakeAuthConfigsRaw) ObjectFactory() objectclient.ObjectFactory { panic("not called") }
