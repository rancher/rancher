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
		cfg   string
		token string
		err   error
	)

	if cluster.LocalClusterAuthEndpoint.Enabled {
		token, err = a.getClusterToken(cluster.ID, apiContext)
	} else {
		token, err = a.getToken(apiContext)
	}
	if err != nil {
		return err
	}

	userName := a.UserMgr.GetUser(apiContext)

	if cluster.LocalClusterAuthEndpoint.Enabled {
		cfg, err = kubeconfig.ForClusterTokenBased(&cluster, apiContext.ID, apiContext.Request.Host, userName, token)
	} else {
		cfg, err = kubeconfig.ForTokenBased(cluster.Name, apiContext.ID, apiContext.Request.Host, userName, token)
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
