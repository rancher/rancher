package publicapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	v3public "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	publicSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3public"
	"github.com/rancher/rancher/pkg/types/config"
)

// NewV1Handler returns an http handler for /v1-public endpoints.
func NewV1Handler(ctx context.Context, scaledContext *config.ScaledContext) (http.Handler, error) {
	providerStore, err := newV1AuthProviderStore(scaledContext.Wrangler)
	if err != nil {
		return nil, fmt.Errorf("creating authprovider store: %w", err)
	}

	authTokenStore := newV1AuthTokenStore(scaledContext.Wrangler)

	r := mux.NewRouter()
	r.Methods(http.MethodGet).Path("/v1-public/authproviders").HandlerFunc(providerStore.List)
	r.Methods(http.MethodPost).Path("/v1-public/login").HandlerFunc(newV1LoginHandler(scaledContext).login)
	r.Methods(http.MethodGet).Path("/v1-public/authtokens/{id}").HandlerFunc(authTokenStore.Get)
	r.Methods(http.MethodDelete).Path("/v1-public/authtokens/{id}").HandlerFunc(authTokenStore.Delete)

	return r, nil
}

type ServerOption func(server *normanapi.Server)

// NewV3Handler returns an http handler for /v3-public endpoints.
// Deprecated. Use NewV1Handler instead. Will be removed in future releases.
func NewV3Handler(ctx context.Context, mgmtCtx *config.ScaledContext, opts ...ServerOption) (http.Handler, error) {
	schemas := types.NewSchemas().AddSchemas(publicSchema.PublicSchemas)
	if err := authProviderSchemas(mgmtCtx, schemas); err != nil {
		return nil, err
	}

	server := normanapi.NewAPIServer()
	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(server)
	}

	return server, nil
}

var authProviderTypes = []string{
	v3public.ActiveDirectoryProviderType,
	v3public.AzureADProviderType,
	v3public.GithubProviderType,
	v3public.GithubAppProviderType,
	v3public.LocalProviderType,
	v3public.OpenLdapProviderType,
	v3public.FreeIpaProviderType,
	v3public.PingProviderType,
	v3public.ADFSProviderType,
	v3public.KeyCloakProviderType,
	v3public.OKTAProviderType,
	v3public.ShibbolethProviderType,
	v3public.GoogleOAuthProviderType,
	v3public.OIDCProviderType,
	v3public.KeyCloakOIDCProviderType,
	v3public.GenericOIDCProviderType,
	v3public.CognitoProviderType,
}

func authProviderSchemas(management *config.ScaledContext, schemas *types.Schemas) error {
	schema := schemas.Schema(&publicSchema.PublicVersion, v3public.AuthProviderType)
	setAuthProvidersStore(schema, management)
	lh := newV3LoginHandler(management)

	for _, apSubtype := range authProviderTypes {
		subSchema := schemas.Schema(&publicSchema.PublicVersion, apSubtype)
		subSchema.Store = subtype.NewSubTypeStore(apSubtype, schema.Store)
		subSchema.ActionHandler = lh.login
		subSchema.Formatter = loginActionFormatter
	}

	schema = schemas.Schema(&publicSchema.PublicVersion, v3public.AuthTokenType)
	setAuthTokensStore(schema, management)
	return nil
}

func loginActionFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "login")
}
