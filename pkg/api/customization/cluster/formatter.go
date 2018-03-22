package clusteregistrationtokens

import (
	"net/http"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/kubeconfig"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/user"
)

func Formatter(request *types.APIContext, resource *types.RawResource) {
	if convert.ToBool(resource.Values["internal"]) {
		delete(resource.Links, "remove")
	}
	shellLink := request.URLBuilder.Link("shell", resource)
	shellLink = strings.Replace(shellLink, "http", "ws", 1)
	shellLink = strings.Replace(shellLink, "/shell", "?shell=true", 1)
	resource.Links["shell"] = shellLink
	resource.AddAction(request, "generateKubeconfig")
}

type ActionHandler struct {
	ClusterClient v3.ClusterInterface
	UserMgr       user.Manager
}

func (a ActionHandler) GenerateKubeconfigActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	if actionName != "generateKubeconfig" {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}

	clusterName := apiContext.ID
	userName := a.UserMgr.GetUser(apiContext)
	token, err := a.UserMgr.EnsureToken("kubeconfig-"+userName, "Kubeconfig token", userName)
	if err != nil {
		return err
	}
	cfg, err := kubeconfig.ForTokenBased(clusterName, apiContext.Request.Host, userName, token)
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
