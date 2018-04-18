package clusteregistrationtokens

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/rancher/rancher/pkg/kubectl"

	"github.com/rancher/rancher/pkg/clustermanager"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/kubeconfig"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementv3 "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/user"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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
	resource.AddAction(request, "importYaml")
}

type ActionHandler struct {
	ClusterClient  v3.ClusterInterface
	UserMgr        user.Manager
	ClusterManager *clustermanager.Manager
}

func (a ActionHandler) ClusterActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case "generateKubeconfig":
		return a.GenerateKubeconfigActionHandler(actionName, action, apiContext)
	case "importYaml":
		return a.ImportYamlHandler(actionName, action, apiContext)
	}
	return httperror.NewAPIError(httperror.NotFound, "not found")
}

func (a ActionHandler) GenerateKubeconfigActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var cluster managementv3.Cluster
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &cluster); err != nil {
		return err
	}

	userName := a.UserMgr.GetUser(apiContext)
	token, err := a.UserMgr.EnsureToken("kubeconfig-"+userName, "Kubeconfig token", userName)
	if err != nil {
		return err
	}
	cfg, err := kubeconfig.ForTokenBased(cluster.Name, apiContext.ID, apiContext.Request.Host, userName, token)
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

func (a ActionHandler) ImportYamlHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	data, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return errors.Wrap(err, "reading request body error")
	}

	input := managementv3.ImportClusterYamlInput{}
	if err = json.Unmarshal(data, &input); err != nil {
		return errors.Wrap(err, "unmarshaling input error")
	}

	var cluster managementv3.Cluster
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &cluster); err != nil {
		return err
	}

	userName := a.UserMgr.GetUser(apiContext)
	cfg, err := a.getKubeConfig(userName, &cluster)
	if err != nil {
		return err
	}
	msg, err := kubectl.Apply([]byte(input.Yaml), cfg)
	if err != nil {
		return err
	}

	rtn := map[string]interface{}{
		"outputMessage": string(msg),
		"type":          "importYamlOutput",
	}
	apiContext.WriteResponse(http.StatusOK, rtn)

	return nil
}

func (a ActionHandler) getKubeConfig(userName string, cluster *managementv3.Cluster) (*clientcmdapi.Config, error) {
	token, err := a.UserMgr.EnsureToken("kubeconfig-"+userName, "token for agent deployment", userName)
	if err != nil {
		return nil, err
	}

	return a.ClusterManager.KubeConfig(cluster.Name, token), nil
}
