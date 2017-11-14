package server

import (
	"context"
	"net/http"
	"net/url"

	"github.com/rancher/cluster-api/api/setup"
	"github.com/rancher/cluster-api/store"
	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	clusterSchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	managementSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	projectSchema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/config"
)

func New(ctx context.Context, cluster *config.ClusterContext) (http.Handler, error) {
	schemas := types.NewSchemas().
		AddSchemas(managementSchema.Schemas).
		AddSchemas(clusterSchema.Schemas).
		AddSchemas(projectSchema.Schemas)

	if err := setup.Schemas(ctx, cluster, schemas); err != nil {
		return nil, err
	}

	server := normanapi.NewAPIServer()
	server.URLParser = func(schemas *types.Schemas, url *url.URL) (parse.ParsedURL, error) {
		return URLParser(cluster.ClusterName, schemas, url)
	}
	server.Resolver = NewResolver(server.Resolver)
	server.StoreWrapper = store.ProjectSetter(server.StoreWrapper)

	if err := server.AddSchemas(schemas); err != nil {
		return nil, err
	}

	return server, nil
}
