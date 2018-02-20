package server

import (
	"context"
	"net/http"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/api/builtin"
	"github.com/rancher/norman/pkg/subscribe"
	"github.com/rancher/rancher/pkg/api/controllers/dynamicschema"
	"github.com/rancher/rancher/pkg/api/controllers/settings"
	"github.com/rancher/rancher/pkg/api/server/managementstored"
	"github.com/rancher/rancher/pkg/api/server/userstored"
	"github.com/rancher/rancher/pkg/rbac"
	clusterSchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectSchema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/config"
)

func New(ctx context.Context, apiContext *config.ScaledContext) (http.Handler, error) {
	subscribe.Register(&builtin.Version, apiContext.Schemas)
	subscribe.Register(&managementSchema.Version, apiContext.Schemas)
	subscribe.Register(&clusterSchema.Version, apiContext.Schemas)
	subscribe.Register(&projectSchema.Version, apiContext.Schemas)

	if err := managementstored.Setup(ctx, apiContext); err != nil {
		return nil, err
	}

	if err := userstored.Setup(ctx, apiContext); err != nil {
		return nil, err
	}

	server := normanapi.NewAPIServer()
	server.AccessControl = rbac.NewAccessControl(apiContext.RBAC)

	if err := server.AddSchemas(apiContext.Schemas); err != nil {
		return nil, err
	}

	dynamicschema.Register(apiContext, server.Schemas)
	err := settings.Register(apiContext)

	return server, err
}
