package server

import (
	"context"
	"net/http"

	"github.com/rancher/management-api/api/setup"
	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/types"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/config"
)

func New(ctx context.Context, cluster *config.ManagementContext) (http.Handler, error) {
	schemas := types.NewSchemas().
		AddSchemas(managementSchema.Schemas)

	if err := setup.Schemas(ctx, cluster, schemas); err != nil {
		return nil, err
	}

	server := normanapi.NewAPIServer()

	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}

	return server, nil
}
