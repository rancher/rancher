package k8slookup

import (
	"net/http"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/clusterrouter"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
)

func New(context *config.ScaledContext, validate bool) clusterrouter.ClusterLookup {
	return &lookup{
		clusterLister: context.Management.Clusters("").Controller().Lister(),
		schemas:       context.Schemas,
		validate:      validate,
	}
}

type lookup struct {
	clusterLister v3.ClusterLister
	schemas       *types.Schemas
	validate      bool
}

func (l *lookup) Lookup(req *http.Request) (*v3.Cluster, error) {
	apiContext := types.NewAPIContext(req, nil, l.schemas)
	clusterID := clusterrouter.GetClusterID(req)
	if clusterID == "" {
		return nil, httperror.NewAPIError(httperror.NotFound, "failed to find cluster")
	}

	if l.validate {
		// check access
		if err := access.ByID(apiContext, &schema.Version, client.ClusterType, clusterID, &client.Cluster{}); err != nil {
			return nil, err
		}
	}

	return l.clusterLister.Get("", clusterID)
}
