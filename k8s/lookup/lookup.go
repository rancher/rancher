package lookup

import (
	"net/http"

	"github.com/rancher/netes/cluster"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
)

func New(context *config.ManagementContext) cluster.Lookup {
	return &lookup{
		clusterLister: context.Management.Clusters("").Controller().Lister(),
		schemas:       context.Schemas,
	}
}

type lookup struct {
	clusterLister v3.ClusterLister
	schemas       *types.Schemas
}

func (l *lookup) Lookup(req *http.Request) (*v3.Cluster, error) {
	apiContext := types.NewAPIContext(req, nil, l.schemas)
	clusterID := cluster.GetClusterID(req)
	if clusterID == "" {
		return nil, httperror.NewAPIError(httperror.NotFound, "failed to find cluster")
	}

	// check access
	if err := access.ByID(apiContext, &schema.Version, client.ClusterType, clusterID, &client.Cluster{}); err != nil {
		return nil, err
	}

	return l.clusterLister.Get("", clusterID)
}
