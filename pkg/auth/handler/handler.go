// Package handler provides HTTP handlers for OIDC authentication flows in Rancher.
//
// This package implements the initial redirect phase of the OIDC authentication flow,
// where users are redirected from Rancher to their configured identity provider (IdP).
//
// The main functionality includes:
//   - Routing HTTP requests to appropriate OIDC providers based on provider name
//   - Retrieving provider configuration from the cluster's auth config
//   - Validating and applying PKCE (Proof Key for Code Exchange) when configured
//   - Constructing and executing redirects to the IdP's authorization endpoint
//
// Supported PKCE methods:
//   - S256: SHA-256 based challenge (recommended)
package handler

import (
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type authConfigGetter interface {
	Get(name string, opts metav1.GetOptions) (runtime.Object, error)
}

// AuthProviderServer handles requests to redirect to OIDC providers.
type AuthProviderServer struct {
	authConfigs authConfigGetter
}

// NewFromAuthConfigInterface provides a simplified interface for creating the
// AuthProviderService.
func NewFromAuthConfigInterface(ac v3.AuthConfigInterface) *AuthProviderServer {
	return NewFromUnstructuredClient(ac.ObjectClient().UnstructuredClient())
}

// NewFromUnstructuredClient creates and returns a new AuthProviderServer.
func NewFromUnstructuredClient(gc authConfigGetter) *AuthProviderServer {
	return &AuthProviderServer{authConfigs: gc}
}

// RegisterOIDCProviderHandlers registers HTTP handlers for OIDC provider redirects on the given mux.
func (p *AuthProviderServer) RegisterOIDCProviderHandlers(mux *mux.Router) {
	mux.HandleFunc("/v1-oidc/{provider}", p.redirectToIdP)
}

// redirectToIdP handles HTTP requests to initiate OIDC authentication by redirecting to the identity provider.
//
// Expected URL format: /v1-oidc/{provider}?state=<state>&scope=<scope>
//
// The authConfig object retrieved must be a runtime.Unstructured object containing:
//   - authEndpoint: The OIDC provider's authorization endpoint URL
//   - clientId: The OAuth2 client ID
//   - rancherUrl: The callback URL for the OIDC flow
//   - pkceMethod (optional): PKCE method ("S256" or "plain") for enhanced security
//
// Query parameters:
//   - state: Authentication state token (passed through to IdP, typically contains CSRF token and redirect info)
//   - scope: OAuth2 scopes requested (e.g., "openid profile email")
//
// If state or scope parameters are missing, they will be passed as empty strings to the IdP.
//
// Security considerations:
//   - The redirect URL is constructed from trusted authConfig data stored in the cluster
//   - User-provided state and scope are passed as query parameters but do not control the redirect destination
//   - PKCE is used when configured to prevent authorization code interception attacks
//   - The PKCE verifier is stored in a secure cookie when PKCE is enabled
func (p *AuthProviderServer) redirectToIdP(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	provider := vars["provider"]

	logrus.Debugf("[oidc] Redirecting to IdP for provider: %s", provider)

	authConfig, err := p.authConfigs.Get(provider, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Debugf("[oidc] Provider not found: %s", provider)
			http.NotFound(w, req)
			return
		}
		logrus.Errorf("[oidc] Failed to get provider configuration for %s: %v", provider, err)
		http.Error(w, "Failed to get provider configuration", http.StatusBadRequest)
		return
	}

	authConfigData, ok := authConfig.(runtime.Unstructured)
	if !ok {
		logrus.Errorf("[oidc] Invalid auth config format for provider %s: expected runtime.Unstructured", provider)
		http.Error(w, "Invalid auth config format", http.StatusInternalServerError)
		return
	}
	data := authConfigData.UnstructuredContent()
	logrus.Debugf("[oidc] Retrieved auth config for provider: %s", provider)

	// Validate that the provider is enabled
	if enabledRaw := data[client.GenericOIDCConfigFieldEnabled]; enabledRaw != nil {
		enabled, ok := enabledRaw.(bool)
		if !ok {
			logrus.Errorf("[oidc] Invalid enabled field type for provider %s: expected bool, got %T", provider, enabledRaw)
			http.Error(w, "Invalid provider configuration", http.StatusInternalServerError)
			return
		}
		if !enabled {
			logrus.Debugf("[oidc] Provider %s is disabled", provider)
			http.Error(w, "Provider is disabled", http.StatusNotFound)
			return
		}
	}

	// Validate PKCE method if configured
	var pkceVerifier string
	if pkceMethodRaw := data[client.GenericOIDCConfigFieldPKCEMethod]; pkceMethodRaw != nil {
		pkceMethod, ok := pkceMethodRaw.(string)
		if !ok {
			logrus.Errorf("[oidc] Invalid PKCE method type for provider %s: expected string, got %T", provider, pkceMethodRaw)
			http.Error(w, "Invalid PKCE method type", http.StatusInternalServerError)
			return
		}

		// Validate supported PKCE methods
		if pkceMethod != "" && pkceMethod != oidc.PKCES256Method {
			logrus.Warnf("[oidc] Unsupported PKCE method '%s' for provider %s", pkceMethod, provider)
			http.Error(w, "Unsupported PKCE method. Supported methods: S256", http.StatusBadRequest)
			return
		}

		if pkceMethod != "" {
			logrus.Debugf("[oidc] Enabling PKCE with method '%s' for provider %s", pkceMethod, provider)
			pkceVerifier = oauth2.GenerateVerifier()
			oidc.SetPKCEVerifier(req, w, pkceVerifier)
		} else {
			logrus.Debugf("[oidc] PKCE not configured for provider %s", provider)
		}
	}
	values := url.Values{
		"state": []string{req.URL.Query().Get("state")},
		"scope": []string{req.URL.Query().Get("scope")},
	}

	redirectURL := oidc.GetOIDCRedirectionURL(data, pkceVerifier, &values)
	logrus.Debugf("[oidc] Redirecting to IdP for provider %s: %s", provider, redirectURL)

	http.Redirect(w, req, redirectURL, http.StatusFound)
}
