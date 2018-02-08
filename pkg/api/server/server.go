package server

import (
	"context"
	"net/http"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/api/builtin"
	"github.com/rancher/norman/pkg/subscribe"
	"github.com/rancher/rancher/pkg/api/controllers/dynamicschema"
	"github.com/rancher/rancher/pkg/api/controllers/listener"
	"github.com/rancher/rancher/pkg/api/controllers/settings"
	"github.com/rancher/rancher/pkg/api/server/managementstored"
	"github.com/rancher/rancher/pkg/api/server/userstored"
	"github.com/rancher/rancher/pkg/rbac"
	clusterSchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectSchema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/config"
)

func New(ctx context.Context, httpPort, httpsPort int, management *config.ManagementContext, getter listener.HandlerGetter) (http.Handler, error) {
	subscribe.Register(&builtin.Version, management.Schemas)
	subscribe.Register(&managementSchema.Version, management.Schemas)
	subscribe.Register(&clusterSchema.Version, management.Schemas)
	subscribe.Register(&projectSchema.Version, management.Schemas)

	if err := managementstored.Setup(ctx, management); err != nil {
		return nil, err
	}

	if err := userstored.Setup(ctx, management); err != nil {
		return nil, err
	}

	server := normanapi.NewAPIServer()
	server.AccessControl = rbac.NewAccessControl(management.RBAC)

	if err := server.AddSchemas(management.Schemas); err != nil {
		return nil, err
	}

	dynamicschema.Register(management, server.Schemas)
	listener.Register(ctx, management, httpPort, httpsPort, getter)
	err := settings.Register(management)

	return server, err
}
