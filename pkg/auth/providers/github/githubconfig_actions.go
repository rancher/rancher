package github

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"

	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
)

func ConfigFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "configureTest")
	resource.AddAction(apiContext, "testAndApply")
}

func (g *GProvider) ConfigActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == "configureTest" {
		return g.configureTest(actionName, action, request)
	} else if actionName == "testAndApply" {
		return g.testAndApply(actionName, action, request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (g *GProvider) configureTest(actionName string, action *types.Action, request *types.APIContext) error {
	githubConfig := &v3.GithubConfig{}
	if err := json.NewDecoder(request.Request.Body).Decode(githubConfig); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}
	redirectURL := FormGithubRedirectURL(githubConfig)

	data := map[string]interface{}{
		"redirectUrl": redirectURL,
		"type":        "githubConfigTestOutput",
	}

	request.WriteResponse(http.StatusOK, data)
	return nil
}

func FormGithubRedirectURL(githubConfig *v3.GithubConfig) string {
	redirect := ""
	if githubConfig.Hostname != "" {
		scheme := "http://"
		if githubConfig.TLS {
			scheme = "https://"
		}
		redirect = scheme + githubConfig.Hostname
	} else {
		redirect = githubDefaultHostName
	}
	redirect = redirect + "/login/oauth/authorize?client_id=" + githubConfig.ClientID

	return redirect
}

func (g *GProvider) testAndApply(actionName string, action *types.Action, request *types.APIContext) error {
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
	err = g.SaveGithubConfig(&githubConfig)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to save github config: %v", err))
	}

	user, err := g.userMGR.SetPrincipalOnCurrentUser(request, userPrincipal)
	if err != nil {
		return err
	}

	//create a new token, set this token as the cookie and return 200
	token := tokens.GenerateNewLoginToken(user.Name, userPrincipal, groupPrincipals, providerInfo, 0, "Token via Github Configuration")
	token, err = tokens.CreateTokenCR(&token)
	if err != nil {
		logrus.Errorf("Failed creating token with error: %v", err)
		return httperror.NewAPIErrorLong(status, "", fmt.Sprintf("Failed creating token with error: %v", err))
	}

	isSecure := false
	if request.Request.URL.Scheme == "https" {
		isSecure = true
	}

	tokenCookie := &http.Cookie{
		Name:     tokens.CookieName,
		Value:    token.ObjectMeta.Name + ":" + token.Token,
		Secure:   isSecure,
		Path:     "/",
		HttpOnly: true,
	}
	http.SetCookie(request.Response, tokenCookie)
	request.WriteResponse(http.StatusOK, nil)

	return nil
}
