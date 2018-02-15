package publicapi

import (
	"context"
	"net/http"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	publicSchema "github.com/rancher/types/apis/management.cattle.io/v3public/schema"
	v3public "github.com/rancher/types/client/management/v3public"
	"github.com/rancher/types/config"
)

func NewHandler(ctx context.Context, mgmtCtx *config.ManagementContext) (http.Handler, error) {
	schemas := types.NewSchemas().AddSchemas(publicSchema.PublicSchemas)
	if err := authProviderSchemas(ctx, mgmtCtx, schemas); err != nil {
		return nil, err
	}

	server := normanapi.NewAPIServer()
	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}
	return server, nil
}

var authProviderTypes = []string{v3public.LocalProviderType, v3public.GithubProviderType, v3public.ActiveDirectoryProviderType}

func authProviderSchemas(ctx context.Context, management *config.ManagementContext, schemas *types.Schemas) error {
	schema := schemas.Schema(&publicSchema.PublicVersion, v3public.AuthProviderType)
	setAuthProvidersStore(schema, management)
	lh := newLoginHandler(management)

	for _, apSubtype := range authProviderTypes {
		subSchema := schemas.Schema(&publicSchema.PublicVersion, apSubtype)
		subSchema.Store = subtype.NewSubTypeStore(apSubtype, schema.Store)
		subSchema.ActionHandler = lh.login
		subSchema.Formatter = loginActionFormatter
	}

	return nil
}

func loginActionFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "login")
}
