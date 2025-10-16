package principals

import (
	"context"
	"net/url"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/requests"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

func Schema(ctx context.Context, management *config.ScaledContext, schemas *types.Schemas, authToken requests.AuthTokenGetter) error {
	p := newPrincipalsHandler(ctx, management, authToken)
	schema := schemas.Schema(&managementSchema.Version, client.PrincipalType)
	schema.ActionHandler = p.actions
	schema.ListHandler = p.list
	schema.CollectionFormatter = collectionFormatter
	schema.Formatter = formatter
	return nil
}

func collectionFormatter(apiContext *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(apiContext, "search")
}

func formatter(request *types.APIContext, resource *types.RawResource) {
	schema := request.Schemas.Schema(&managementSchema.Version, client.PrincipalType)
	resource.Links = map[string]string{"self": request.URLBuilder.ResourceLinkByID(schema, url.PathEscape(resource.ID))}
}
