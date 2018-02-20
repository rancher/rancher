package tokens

import (
	"context"
	"net/http"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/types"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
)

var crdVersions = []*types.APIVersion{
	&managementSchema.Version,
}

func NewAPIHandler(ctx context.Context, apiContext *config.ScaledContext) (http.Handler, error) {
	err := NewTokenAPIServer(ctx, apiContext)
	if err != nil {
		return nil, err
	}

	schemas := types.NewSchemas().AddSchemas(managementSchema.TokenSchemas)

	if err := tokenSchema(ctx, schemas); err != nil {
		return nil, err
	}

	server := normanapi.NewAPIServer()

	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}

	return server, nil
}

func tokenSchema(ctx context.Context, schemas *types.Schemas) error {
	schema := schemas.Schema(&managementSchema.Version, client.TokenType)
	schema.CollectionActions = map[string]types.Action{
		"logout": {},
	}
	schema.ActionHandler = tokenActionHandler
	schema.ListHandler = tokenListHandler
	schema.CreateHandler = tokenCreateHandler
	schema.DeleteHandler = tokenDeleteHandler

	return nil
}
