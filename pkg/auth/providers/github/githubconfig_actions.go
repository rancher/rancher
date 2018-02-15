package github

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"

	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/client/management/v3"
)

func (g *ghProvider) formatter(apiContext *types.APIContext, resource *types.RawResource) {
	common.AddCommonActions(apiContext, resource)
	resource.AddAction(apiContext, "configureTest")
	resource.AddAction(apiContext, "testAndApply")
}

func (g *ghProvider) actionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	handled, err := common.HandleCommonAction(actionName, action, request, Name, g.authConfigs)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	if actionName == "configureTest" {
		return g.configureTest(actionName, action, request)
	} else if actionName == "testAndApply" {
		return g.testAndApply(actionName, action, request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (g *ghProvider) configureTest(actionName string, action *types.Action, request *types.APIContext) error {
	githubConfig := &v3.GithubConfig{}
	if err := json.NewDecoder(request.Request.Body).Decode(githubConfig); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}
	redirectURL := formGithubRedirectURL(githubConfig)

	data := map[string]interface{}{
		"redirectUrl": redirectURL,
		"type":        "githubConfigTestOutput",
	}

	request.WriteResponse(http.StatusOK, data)
	return nil
}

func formGithubRedirectURL(githubConfig *v3.GithubConfig) string {
	return githubRedirectURL(githubConfig.Hostname, githubConfig.ClientID, githubConfig.TLS)
}

func formGithubRedirectURLFromMap(config map[string]interface{}) string {
	hostname, _ := config[client.GithubConfigFieldHostname].(string)
	clientID, _ := config[client.GithubConfigFieldClientID].(string)
	tls, _ := config[client.GithubConfigFieldTLS].(bool)
	return githubRedirectURL(hostname, clientID, tls)
}

func githubRedirectURL(hostname, clientID string, tls bool) string {
	redirect := ""
	if hostname != "" {
		scheme := "http://"
		if tls {
			scheme = "https://"
		}
		redirect = scheme + hostname
	} else {
		redirect = githubDefaultHostName
	}
	redirect = redirect + "/login/oauth/authorize?client_id=" + clientID
	return redirect
}

func (g *ghProvider) testAndApply(actionName string, action *types.Action, request *types.APIContext) error {
	var githubConfig v3.GithubConfig
	githubConfigApplyInput := &v3.GithubConfigApplyInput{}

	if err := json.NewDecoder(request.Request.Body).Decode(githubConfigApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	githubConfig = githubConfigApplyInput.GithubConfig
	githubLogin := &v3public.GithubLogin{
		Code: githubConfigApplyInput.Code,
	}

	//Call provider to testLogin
	userPrincipal, groupPrincipals, providerInfo, status, err := g.LoginUser(githubLogin, &githubConfig)
	if err != nil {
		if status == 0 || status == 500 {
			status = http.StatusInternalServerError
			return httperror.NewAPIErrorLong(status, "ServerError", fmt.Sprintf("Failed to login to github: %v", err))
		}
		return httperror.NewAPIErrorLong(status, "",
			fmt.Sprintf("Failed to login to github: %v", err))
	}

	//if this works, save githubConfig CR adding enabled flag
	githubConfig.Enabled = githubConfigApplyInput.Enabled
	err = g.saveGithubConfig(&githubConfig)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to save github config: %v", err))
	}

	user, err := g.userMGR.SetPrincipalOnCurrentUser(request, userPrincipal)
	if err != nil {
		return err
	}

	return tokens.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerInfo, 0, "Token via Githu Configuration", request)
}
