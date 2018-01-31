package server

import (
	"context"
	"net/http"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/types"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/config"

	"github.com/rancher/rancher/pkg/auth/api/setup"
	"github.com/rancher/rancher/pkg/auth/tokens"
)

var crdVersions = []*types.APIVersion{
	&managementSchema.Version,
}

func NewTokenAPIHandler(ctx context.Context, mgmtCtx *config.ManagementContext) (http.Handler, error) {
	err := tokens.NewTokenAPIServer(ctx, mgmtCtx)
	if err != nil {
		return nil, err
	}

	handler, err := newHandler(ctx, mgmtCtx)
	if err != nil {
		return nil, err
	}
	return handler, nil
}

func newHandler(ctx context.Context, mgmtCtx *config.ManagementContext) (http.Handler, error) {
	schemas := types.NewSchemas().AddSchemas(managementSchema.Schemas)

	if err := setup.Schemas(ctx, mgmtCtx, schemas); err != nil {
		return nil, err
	}

	server := normanapi.NewAPIServer()

	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}
	return server, nil
}
