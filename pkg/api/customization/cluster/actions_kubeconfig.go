package cluster

import (
	"net/http"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/kubeconfig"
	mgmtclient "github.com/rancher/types/client/management/v3"
)

func (a ActionHandler) GenerateKubeconfigActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var cluster mgmtclient.Cluster
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &cluster); err != nil {
		return err
	}

	var (
		cfg string
		err error
	)
	endpointEnabled := cluster.LocalClusterAuthEndpoint != nil && cluster.LocalClusterAuthEndpoint.Enabled

	if endpointEnabled {
		cfg, err = kubeconfig.ForClusterTokenBased(&cluster, apiContext.ID, apiContext.Request.Host)
	} else {
		cfg, err = kubeconfig.ForTokenBased(cluster.Name, apiContext.ID, apiContext.Request.Host)
	}
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"config": cfg,
		"type":   "generateKubeconfigOutput",
	}
	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}
