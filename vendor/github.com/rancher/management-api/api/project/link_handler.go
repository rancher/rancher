package project

import (
	"strings"

	"github.com/rancher/management-api/cluster"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
)

func NewClusterRouterLinkHandler(manager *cluster.Manager) types.RequestHandler {
	return func(apiContext *types.APIContext) error {
		return clusterRouterLinkHandler(apiContext, manager)
	}
}

func clusterRouterLinkHandler(apiContext *types.APIContext, clusterManager *cluster.Manager) error {
	clusterID := apiContext.ID
	if apiContext.Type == client.ProjectType {
		project := &client.Project{}
		if err := access.ByID(apiContext, &schema.Version, client.ProjectType, apiContext.ID, project); err != nil {
			return err
		}
		clusterID = project.ClusterId
	}

	cluster := &client.Cluster{}
	if err := access.ByID(apiContext, &schema.Version, client.ClusterType, clusterID, cluster); err != nil {
		return err
	}

	handler := clusterManager.APIServer(apiContext.Request.Context(), cluster)
	if handler == nil {
		return httperror.NewAPIError(httperror.NotFound, "failed to find cluster")
	}

	parts := strings.SplitN(apiContext.Request.URL.Path, "/", 5)
	if len(parts) > 4 {
		parts[4] = strings.Replace(parts[4], "namespaced", "", 1)
		apiContext.Request.URL.Path = strings.Join(parts, "/")
	}

	handler.ServeHTTP(apiContext.Response, apiContext.Request)
	return nil
}
