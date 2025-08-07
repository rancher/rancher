package api

import (
	"context"
	"net/http"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/pkg/subscribe"
	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/pkg/auth/api/user"
	"github.com/rancher/rancher/pkg/auth/principals"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	"github.com/rancher/rancher/pkg/auth/requests"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

func Setup(ctx context.Context, scaledContext *config.ScaledContext, schemas *types.Schemas, authToken requests.AuthTokenGetter) {
	principals.Schema(ctx, scaledContext, schemas, authToken)
	providers.SetupAuthConfig(ctx, scaledContext, schemas)
	user.SetUserStore(schemas.Schema(&managementschema.Version, client.UserType), scaledContext)
	User(ctx, schemas, scaledContext)
}

func User(ctx context.Context, schemas *types.Schemas, management *config.ScaledContext) {
	extTokenStore := exttokenstore.NewSystemFromWrangler(management.Wrangler)

	schema := schemas.Schema(&managementschema.Version, client.UserType)
	handler := &user.Handler{
		UserClient:               management.Management.Users(""),
		GlobalRoleBindingsClient: management.Management.GlobalRoleBindings(""),
		UserAuthRefresher:        providerrefresh.NewUserAuthRefresher(management),
		ExtTokenStore:            extTokenStore,
		SecretLister:             management.Wrangler.Core.Secret().Cache(),
		SecretClient:             management.Wrangler.Core.Secret(),
		PwdChanger:               pbkdf2.New(management.Wrangler.Core.Secret().Cache(), management.Wrangler.Core.Secret()),
	}

	schema.Formatter = handler.UserFormatter
	schema.CollectionFormatter = handler.CollectionFormatter
	schema.ActionHandler = handler.Actions
}

func NewNormanServer(ctx context.Context, scaledContext *config.ScaledContext, authToken requests.AuthTokenGetter) (http.Handler, error) {
	schemas, err := newSchemas(ctx, scaledContext)
	if err != nil {
		return nil, err
	}

	Setup(ctx, scaledContext, schemas, authToken)

	server := normanapi.NewAPIServer()
	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}
	return server, nil
}

func newSchemas(ctx context.Context, apiContext *config.ScaledContext) (*types.Schemas, error) {
	schemas := types.NewSchemas()
	schemas.AddSchemas(managementschema.AuthSchemas)
	subscribe.Register(&managementschema.Version, schemas)

	factory := &crd.Factory{ClientGetter: apiContext.ClientGetter}
	factory.BatchCreateCRDs(ctx, config.ManagementStorageContext, scheme.Scheme, schemas, &managementschema.Version,
		client.AuthConfigType,
		client.GroupMemberType,
		client.GroupType,
		client.TokenType,
		client.UserAttributeType,
		client.UserType,
	)

	return schemas, factory.BatchWait()
}
