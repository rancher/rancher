package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rancher/norman/api/builtin"
	"github.com/rancher/norman/pkg/subscribe"
	rancherapi "github.com/rancher/rancher/pkg/api"
	"github.com/rancher/rancher/pkg/api/controllers/catalog"
	"github.com/rancher/rancher/pkg/api/controllers/dynamicschema"
	"github.com/rancher/rancher/pkg/api/controllers/feature"
	"github.com/rancher/rancher/pkg/api/controllers/k3smetadata"
	"github.com/rancher/rancher/pkg/api/controllers/samlconfig"
	"github.com/rancher/rancher/pkg/api/controllers/settings"
	"github.com/rancher/rancher/pkg/api/controllers/usercontrollers"
	whitelistproxyKontainerDriver "github.com/rancher/rancher/pkg/api/controllers/whitelistproxy/kontainerdriver"
	whitelistproxyNodeDriver "github.com/rancher/rancher/pkg/api/controllers/whitelistproxy/nodedriver"
	"github.com/rancher/rancher/pkg/api/server/managementstored"
	"github.com/rancher/rancher/pkg/api/server/userstored"
	"github.com/rancher/rancher/pkg/clustermanager"
	clusterSchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectSchema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/config"
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
	if err := managementstored.AddOpenAPIV3SchemaToCRD(ctx, scaledContext); err != nil {
		fmt.Printf("\nerr adding openapi schema: %v\n", err)
	}

	if err := userstored.Setup(ctx, scaledContext, clusterManager, k8sProxy); err != nil {
		return nil, err
	}

	server, err := rancherapi.NewServer(scaledContext.Schemas)
	if err != nil {
		return nil, err
	}
	server.AccessControl = scaledContext.AccessControl

	catalog.Register(ctx, scaledContext)
	dynamicschema.Register(ctx, scaledContext, server.Schemas)
	feature.Register(ctx, scaledContext)
	whitelistproxyNodeDriver.Register(ctx, scaledContext)
	whitelistproxyKontainerDriver.Register(ctx, scaledContext)
	samlconfig.Register(ctx, scaledContext)
	k3smetadata.Register(ctx, scaledContext)
	usercontrollers.Register(ctx, scaledContext, clusterManager)
	err = settings.Register(scaledContext)

	return server, err
}
