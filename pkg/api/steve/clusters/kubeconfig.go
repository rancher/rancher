package clusters

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/kubeconfig"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type kubeconfigDownload struct {
	userMgr user.Manager
}

func (k kubeconfigDownload) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	apiRequest := types.GetAPIContext(req.Context())
	if err := apiRequest.AccessControl.CanGet(apiRequest, apiRequest.Schema); err != nil {
		apiRequest.WriteError(err)
		return
	}

	if features.MCM.Enabled() {
		http.Redirect(rw, req, fmt.Sprintf("/v3/clusters/%s?action=generateKubeconfig", apiRequest.Name), http.StatusFound)
		return
	}

	userName, ok := request.UserFrom(req.Context())
	if !ok {
		apiRequest.WriteError(validation.Unauthorized)
		return
	}
	var tokenKey string
	var err error
	generateToken := strings.EqualFold(settings.KubeconfigGenerateToken.Get(), "true")
	if generateToken {
		tokenKey, err = k.ensureToken(userName.GetName())
		if err != nil {
			apiRequest.WriteError(err)
			return
		}
	}

	host := settings.ServerURL.Get()
	if host == "" {
		host = apiRequest.Request.Host
	} else {
		u, err := url.Parse(host)
		if err == nil {
			host = u.Host
		} else {
			host = apiRequest.Request.Host
		}
	}
	cfg, err := kubeconfig.ForTokenBased(apiRequest.Name, apiRequest.Name, host, tokenKey)
	if err != nil {
		apiRequest.WriteError(err)
		return
	}
	apiRequest.WriteResponse(http.StatusOK, types.APIObject{
		Type: "generateKubeconfigOutput",
		Object: &GenerateKubeconfigOutput{
			Config: cfg,
		},
	})
}

func (k kubeconfigDownload) ensureToken(userName string) (string, error) {
	tokenNamePrefix := fmt.Sprintf("kubeconfig-%s", userName)
	token, err := k.userMgr.EnsureToken(tokenNamePrefix, "Kubeconfig token", "kubeconfig", userName, nil, true)
	return token, err
}
