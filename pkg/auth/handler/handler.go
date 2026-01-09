package handler

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
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

func (p *AuthProviderServer) redirectToIdP(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	authConfig, err := p.authConfigs.Get(vars["provider"], metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			http.NotFound(w, req)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to get provider %v", vars["provider"]), http.StatusBadRequest)
		return
	}

	authConfigData, ok := authConfig.(runtime.Unstructured)
	if !ok {
		http.Error(w, "Invalid auth config format", http.StatusInternalServerError)
		return
	}
	data := authConfigData.UnstructuredContent()
	pkceMethod := data[client.GenericOIDCConfigFieldPKCEMethod]
	var pkceVerifier string
	if pkceMethod == oidc.PKCES256Method {
		pkceVerifier = oauth2.GenerateVerifier()
		oidc.SetPKCEVerifier(req, w, pkceVerifier)
	}
	values := url.Values{
		"state": []string{req.URL.Query().Get("state")},
		"scope": []string{req.URL.Query().Get("scope")},
	}

	redirectURL := oidc.GetOIDCRedirectionURL(data, pkceVerifier, &values)

	http.Redirect(w, req, redirectURL, http.StatusFound)
}
