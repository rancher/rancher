package provider

import (
	"context"
	"time"

	"github.com/gorilla/mux"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	wrangmgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/oidc/provider/session"
	"github.com/rancher/rancher/pkg/oidc/randomstring"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	oidcClientByIDIndex = "oidc.management.cattle.io/oidcclient-by-id"
	secretsNamespace    = "cattle-oidc-client-secrets"
	codesNamespace      = "cattle-oidc-codes"
	maxTime             = 10 * time.Minute
)

type Provider struct {
	jwksHandler     *jwksHandler
	authHandler     *authorizeHandler
	tokenHandler    *tokenHandler
	userInfoHandler *userInfoHandler
}

func NewProvider(ctx context.Context, tokenCache wrangmgmtv3.TokenCache, tokenClient wrangmgmtv3.TokenClient, userLister wrangmgmtv3.UserCache, userAttributeLister wrangmgmtv3.UserAttributeCache, secretCache corecontrollers.SecretCache, secretClient corecontrollers.SecretClient, oidcClientCache wrangmgmtv3.OIDCClientCache, oidcClientController wrangmgmtv3.OIDCClientController, namespaceClient corecontrollers.NamespaceClient) (Provider, error) {
	sessionStorage := session.NewSecretSessionStore(ctx, secretCache, secretClient, maxTime)
	jwks, err := newJWKSHandler(secretCache, secretClient)
	if err != nil {
		return Provider{}, err
	}
	oidcClientInformer := oidcClientController.Informer()
	oidcClientIndexers := map[string]cache.IndexFunc{
		oidcClientByIDIndex: func(obj interface{}) ([]string, error) {
			o, ok := obj.(*v3.OIDCClient)
			if !ok {
				return []string{}, nil
			}

			return []string{o.Status.ClientID}, nil
		},
	}
	err = oidcClientInformer.AddIndexers(oidcClientIndexers)
	if err != nil {
		return Provider{}, err
	}

	// create necessary namespaces
	if _, err := namespaceClient.Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: secretsNamespace,
		},
	}); err != nil && !apierrors.IsAlreadyExists(err) {
		return Provider{}, err
	}
	if _, err := namespaceClient.Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: codesNamespace,
		},
	}); err != nil && !apierrors.IsAlreadyExists(err) {
		return Provider{}, err
	}

	return Provider{
		jwksHandler:     jwks,
		authHandler:     newAuthorizeHandler(tokenCache, userLister, sessionStorage, &randomstring.Generator{}, oidcClientCache),
		tokenHandler:    newTokenHandler(tokenCache, userLister, userAttributeLister, sessionStorage, jwks, oidcClientCache, oidcClientController, secretCache, tokenClient),
		userInfoHandler: newUserInfoHandler(userLister, userAttributeLister, jwks),
	}, nil
}

// RegisterOIDCProviderHandles register all Handlers for the OIDC provider.
func (p *Provider) RegisterOIDCProviderHandles(mux *mux.Router) {
	mux.HandleFunc("/oidc/.well-known/openid-configuration", openIDConfigurationEndpoint)
	mux.HandleFunc("/oidc/.well-known/jwks.json", p.jwksHandler.jwksEndpoint)
	mux.HandleFunc("/oidc/authorize", p.authHandler.authEndpoint)
	mux.HandleFunc("/oidc/token", p.tokenHandler.tokenEndpoint)
	mux.HandleFunc("/oidc/userinfo", p.userInfoHandler.userInfoEndpoint)
}
