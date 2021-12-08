package publicapi

import (
	"context"
	"net/http"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	v3public "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	publicSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3public"
	"github.com/rancher/rancher/pkg/types/config"
)

type ServerOption func(server *normanapi.Server)

func NewHandler(ctx context.Context, mgmtCtx *config.ScaledContext, opts ...ServerOption) (http.Handler, error) {
	schemas := types.NewSchemas().AddSchemas(publicSchema.PublicSchemas)
	if err := authProviderSchemas(ctx, mgmtCtx, schemas); err != nil {
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
}

func authProviderSchemas(ctx context.Context, management *config.ScaledContext, schemas *types.Schemas) error {
	schema := schemas.Schema(&publicSchema.PublicVersion, v3public.AuthProviderType)
	setAuthProvidersStore(schema, management)
	lh := newLoginHandler(ctx, management)

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
