package server

import (
	"context"
	"net/http"

	"github.com/rancher/management-api/api/setup"
	"github.com/rancher/management-api/controller/dynamicschema"
	"github.com/rancher/norman-rbac"
	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/types/config"
)

func New(ctx context.Context, management *config.ManagementContext) (http.Handler, error) {
	if err := setup.Schemas(ctx, management, management.Schemas); err != nil {
		return nil, err
	}

	server := normanapi.NewAPIServer()
	server.AccessControl = rbac.NewAccessControl(management.RBAC)

	if err := server.AddSchemas(management.Schemas); err != nil {
		return nil, err
	}

	dynamicschema.Register(management, server.Schemas)

	return server, nil
}
