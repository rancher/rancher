package server

import (
	"context"
	"net/http"

	"github.com/rancher/rancher/pkg/api/norman"

	"github.com/rancher/norman/api/builtin"
	"github.com/rancher/norman/pkg/subscribe"
	"github.com/rancher/rancher/pkg/api/norman/server/managementstored"
	"github.com/rancher/rancher/pkg/api/norman/server/userstored"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/managementapi"
	clusterSchema "github.com/rancher/rancher/pkg/schemas/cluster.cattle.io/v3"
	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	projectSchema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

func New(ctx context.Context, scaledContext *config.ScaledContext, clusterManager *clustermanager.Manager,
	k8sProxy http.Handler, localClusterEnabled bool) (http.Handler, error) {
	subscribe.Register(&builtin.Version, scaledContext.Schemas)
	subscribe.Register(&managementSchema.Version, scaledContext.Schemas)
	subscribe.Register(&clusterSchema.Version, scaledContext.Schemas)
	subscribe.Register(&projectSchema.Version, scaledContext.Schemas)

	if err := managementstored.Setup(ctx, scaledContext, clusterManager, k8sProxy, localClusterEnabled); err != nil {
		return nil, err
	}

	if err := userstored.Setup(ctx, scaledContext, clusterManager, k8sProxy); err != nil {
		return nil, err
	}

	server, err := norman.NewServer(scaledContext.Schemas)
	if err != nil {
		return nil, err
	}
	server.AccessControl = scaledContext.AccessControl

	return server, managementapi.Register(ctx, scaledContext, clusterManager, server)
}
