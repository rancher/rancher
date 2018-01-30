package project

import (
	"path"

	"github.com/rancher/norman/types"
	clusterSchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	projectSchema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
)

func ListHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	if apiContext.ID == "" {
		return next(apiContext, nil)
	}

	version := projectSchema.Version
	if apiContext.Schema.ID == client.ClusterType {
		version = clusterSchema.Version
	}

	version.Path = path.Join(version.Path, apiContext.ID)
	apiContext.SchemasVersion = &version
	return next(apiContext, nil)
}
