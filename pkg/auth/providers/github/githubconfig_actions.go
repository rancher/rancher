package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
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
		return g.configureTest(request)
	} else if actionName == "testAndApply" {
		return g.testAndApply(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (g *ghProvider) configureTest(request *types.APIContext) error {
	githubConfig := &v32.GithubConfig{}
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

func formGithubRedirectURL(githubConfig *v32.GithubConfig) string {
	return githubRedirectURL(githubConfig.Hostname, githubConfig.ClientID, githubConfig.TLS)
}

func formGithubRedirectURLFromMap(config map[string]interface{}) string {
	hostname, _ := config[client.GithubConfigFieldHostname].(string)
	clientID, _ := config[client.GithubConfigFieldClientID].(string)
	tls, _ := config[client.GithubConfigFieldTLS].(bool)

	requestHostname := convert.ToString(config[".host"])
	clientIDs := convert.ToMapInterface(config["hostnameToClientId"])
	if otherID, ok := clientIDs[requestHostname]; ok {
		clientID = convert.ToString(otherID)
	}
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

func (g *ghProvider) testAndApply(request *types.APIContext) error {
	var githubConfig v32.GithubConfig
	githubConfigApplyInput := &v32.GithubConfigApplyInput{}

	if err := json.NewDecoder(request.Request.Body).Decode(githubConfigApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}
	githubConfig = githubConfigApplyInput.GithubConfig
	githubLogin := &v32.GithubLogin{
		Code: githubConfigApplyInput.Code,
	}

	if githubConfig.ClientSecret != "" {
		value, err := common.ReadFromSecret(g.secrets, githubConfig.ClientSecret,
			strings.ToLower(client.GithubConfigFieldClientSecret))
		if err != nil {
			return err
		}
		githubConfig.ClientSecret = value
	}

	// Call provider to testLogin
	userPrincipal, groupPrincipals, providerInfo, err := g.LoginUser("", githubLogin, &githubConfig, true)
	if err != nil {
		if httperror.IsAPIError(err) {
			return err
		}
		return errors.Wrap(err, "server error while authenticating")
	}

	// if this works, save githubConfig CR adding enabled flag
	user, err := g.userMGR.SetPrincipalOnCurrentUser(request, userPrincipal)
	if err != nil {
		return err
	}

	githubConfig.Enabled = githubConfigApplyInput.Enabled
	err = g.saveGithubConfig(&githubConfig)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to save github config: %v", err))
	}

	userExtraInfo := g.GetUserExtraAttributes(userPrincipal)
	err = g.tokenMGR.UserAttributeCreateOrUpdate(user.Name, userPrincipal.Provider, groupPrincipals, userExtraInfo)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to create or update userAttribute: %v", err))
	}

	return g.tokenMGR.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerInfo, 0, "Token via Github Configuration", request)
}
