package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimetesting "k8s.io/apimachinery/pkg/runtime/testing"
)

func TestAuthenticationProvidersUnknownAuthConfig(t *testing.T) {
	fc := newFakeUnstructuredClient(map[string]*runtimetesting.Unstructured{})
	srv := NewFromUnstructuredClient(fc)
	router := mux.NewRouter()
	srv.RegisterOIDCProviderHandlers(router)
	testSrv := httptest.NewServer(router)
	t.Cleanup(func() {
		testSrv.Close()
	})

	req, err := http.NewRequest(http.MethodGet, testSrv.URL+"/v1-oidc/unknownprovider?state=test&scope=openid", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := testSrv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got Status %v, want %v", resp.StatusCode, http.StatusNotFound)
	}
}

func TestAuthenticationProviders(t *testing.T) {
	fc := newFakeUnstructuredClient(map[string]*runtimetesting.Unstructured{
		"keycloakoidc": &runtimetesting.Unstructured{
			Object: map[string]any{
				"apiVersion": "management.cattle.io/v3",
				"kind":       "AuthConfig",
				"metadata": map[string]any{
					"name": "keycloakoidc",
				},
				"scope":      "openid profile email",
				"type":       "keyCloakOIDCConfig",
				"accessMode": "unrestricted",
				"allowedPrincipalIds": []string{
					"keycloakoidc_user://19b4bc71-afe8-47e9-a4d8-bdae59f8da42",
				},
				"authEndpoint": "https://keycloak.example.com/realms/testing/protocol/openid-connect/auth",
				"clientId":     "rancher",
				"enabled":      true,
				"issuer":       "https://keycloak.example.com/realms/testing",
				"rancherUrl":   "https://rancher.example.com/verify-auth",
			},
		},
	})
	srv := NewFromUnstructuredClient(fc)
	router := mux.NewRouter()
	srv.RegisterOIDCProviderHandlers(router)
	testSrv := httptest.NewServer(router)
	t.Cleanup(func() {
		testSrv.Close()
	})

	req, err := http.NewRequest(http.MethodGet, testSrv.URL+"/v1-oidc/keycloakoidc?state=eyJub25jZSI6Ik03dHVvV2JINzJGUWNzRHIiLCJ0byI6InZ1ZSIsInByb3ZpZGVyIjoia2V5Y2xvYWtvaWRjIn&scope=openid%20email%20profile", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := testSrv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Errorf("got Status %v, want %v", resp.StatusCode, http.StatusFound)
	}
	want := "https://keycloak.example.com/realms/testing/protocol/openid-connect/auth?client_id=rancher&redirect_uri=https%3A%2F%2Francher.example.com%2Fverify-auth&response_type=code&scope=openid+email+profile&state=eyJub25jZSI6Ik03dHVvV2JINzJGUWNzRHIiLCJ0byI6InZ1ZSIsInByb3ZpZGVyIjoia2V5Y2xvYWtvaWRjIn"
	if l := resp.Header.Get("Location"); l != want {
		t.Errorf("got Location header %q, want %q", l, want)
	}
}

func TestAuthenticationProvidersWithPKCE(t *testing.T) {
	fc := newFakeUnstructuredClient(map[string]*runtimetesting.Unstructured{
		"oidc-with-pkce": &runtimetesting.Unstructured{
			Object: map[string]any{
				"apiVersion": "management.cattle.io/v3",
				"kind":       "AuthConfig",
				"metadata": map[string]any{
					"name": "oidc-with-pkce",
				},
				"scope":        "openid profile email",
				"type":         "oidcConfig",
				"authEndpoint": "https://idp.example.com/oauth/authorize",
				"clientId":     "test-client",
				"enabled":      true,
				"issuer":       "https://idp.example.com",
				"rancherUrl":   "https://rancher.example.com/verify-auth",
				"pkceMethod":   "S256",
			},
		},
	})
	srv := NewFromUnstructuredClient(fc)
	router := mux.NewRouter()
	srv.RegisterOIDCProviderHandlers(router)
	testSrv := httptest.NewServer(router)
	t.Cleanup(func() {
		testSrv.Close()
	})

	req, err := http.NewRequest(http.MethodGet, testSrv.URL+"/v1-oidc/oidc-with-pkce?state=teststate&scope=openid%20profile", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := testSrv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Errorf("got Status %v, want %v", resp.StatusCode, http.StatusFound)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		t.Fatal("expected Location header, got empty")
	}

	// Verify PKCE challenge is present in redirect URL
	if !strings.Contains(location, "code_challenge=") {
		t.Error("expected PKCE code_challenge parameter in redirect URL")
	}
	if !strings.Contains(location, "code_challenge_method=S256") {
		t.Error("expected PKCE code_challenge_method=S256 in redirect URL")
	}
}

func TestAuthenticationProvidersInvalidPKCEMethodType(t *testing.T) {
	fc := newFakeUnstructuredClient(map[string]*runtimetesting.Unstructured{
		"invalid-pkce-type": &runtimetesting.Unstructured{
			Object: map[string]any{
				"apiVersion": "management.cattle.io/v3",
				"kind":       "AuthConfig",
				"metadata": map[string]any{
					"name": "invalid-pkce-type",
				},
				"scope":        "openid profile email",
				"type":         "oidcConfig",
				"authEndpoint": "https://idp.example.com/oauth/authorize",
				"clientId":     "test-client",
				"enabled":      true,
				"issuer":       "https://idp.example.com",
				"rancherUrl":   "https://rancher.example.com/verify-auth",
				"pkceMethod":   123, // Invalid: not a string
			},
		},
	})
	srv := NewFromUnstructuredClient(fc)
	router := mux.NewRouter()
	srv.RegisterOIDCProviderHandlers(router)
	testSrv := httptest.NewServer(router)
	t.Cleanup(func() {
		testSrv.Close()
	})

	req, err := http.NewRequest(http.MethodGet, testSrv.URL+"/v1-oidc/invalid-pkce-type?state=teststate&scope=openid", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := testSrv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("got Status %v, want %v", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestAuthenticationProvidersUnsupportedPKCEMethod(t *testing.T) {
	fc := newFakeUnstructuredClient(map[string]*runtimetesting.Unstructured{
		"unsupported-pkce": &runtimetesting.Unstructured{
			Object: map[string]any{
				"apiVersion": "management.cattle.io/v3",
				"kind":       "AuthConfig",
				"metadata": map[string]any{
					"name": "unsupported-pkce",
				},
				"scope":        "openid profile email",
				"type":         "oidcConfig",
				"authEndpoint": "https://idp.example.com/oauth/authorize",
				"clientId":     "test-client",
				"enabled":      true,
				"issuer":       "https://idp.example.com",
				"rancherUrl":   "https://rancher.example.com/verify-auth",
				"pkceMethod":   "SHA512", // Invalid: unsupported method
			},
		},
	})
	srv := NewFromUnstructuredClient(fc)
	router := mux.NewRouter()
	srv.RegisterOIDCProviderHandlers(router)
	testSrv := httptest.NewServer(router)
	t.Cleanup(func() {
		testSrv.Close()
	})

	req, err := http.NewRequest(http.MethodGet, testSrv.URL+"/v1-oidc/unsupported-pkce?state=teststate&scope=openid", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := testSrv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got Status %v, want %v", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAuthenticationProvidersDisabledProvider(t *testing.T) {
	fc := newFakeUnstructuredClient(map[string]*runtimetesting.Unstructured{
		"disabled-provider": &runtimetesting.Unstructured{
			Object: map[string]any{
				"apiVersion": "management.cattle.io/v3",
				"kind":       "AuthConfig",
				"metadata": map[string]any{
					"name": "disabled-provider",
				},
				"scope":        "openid profile email",
				"type":         "oidcConfig",
				"authEndpoint": "https://idp.example.com/oauth/authorize",
				"clientId":     "test-client",
				"enabled":      false, // Provider is disabled
				"issuer":       "https://idp.example.com",
				"rancherUrl":   "https://rancher.example.com/verify-auth",
			},
		},
	})
	srv := NewFromUnstructuredClient(fc)
	router := mux.NewRouter()
	srv.RegisterOIDCProviderHandlers(router)
	testSrv := httptest.NewServer(router)
	t.Cleanup(func() {
		testSrv.Close()
	})

	req, err := http.NewRequest(http.MethodGet, testSrv.URL+"/v1-oidc/disabled-provider?state=teststate&scope=openid", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := testSrv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got Status %v, want %v", resp.StatusCode, http.StatusNotFound)
	}
}

func newFakeUnstructuredClient(acs map[string]*runtimetesting.Unstructured) *fakeUnstructuredClient {
	configs := map[string]runtime.Unstructured{}
	for k, v := range acs {
		configs[k] = v
	}

	return &fakeUnstructuredClient{
		authProviders: configs,
	}
}

type fakeUnstructuredClient struct {
	authProviders map[string]runtime.Unstructured
}

func (f *fakeUnstructuredClient) Get(name string, opts metav1.GetOptions) (runtime.Object, error) {
	if ap, ok := f.authProviders[name]; ok {
		return ap, nil
	}

	return nil, apierrors.NewNotFound(schema.GroupResource{
		Group: "management.cattle.io", Resource: "authconfigs"}, name)
}
