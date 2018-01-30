package project

import (
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/management/cluster"
	"github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
)

func NewClusterRouterLinkHandler(manager *cluster.Manager) types.RequestHandler {
	return func(apiContext *types.APIContext, next types.RequestHandler) error {
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

	handler.ServeHTTP(apiContext.Response, apiContext.Request)
	return nil
}
