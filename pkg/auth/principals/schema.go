package principals

import (
	"context"

	"github.com/rancher/norman/types"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
)

func Schema(ctx context.Context, management *config.ManagementContext, schemas *types.Schemas) error {
	p := newPrincipalsHandler(ctx, management)
	schema := schemas.Schema(&managementSchema.Version, client.PrincipalType)
	schema.ActionHandler = p.actions
	schema.ListHandler = p.list
	schema.CollectionFormatter = collectionFormatter
	return nil
}

func collectionFormatter(apiContext *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(apiContext, "search")
}
