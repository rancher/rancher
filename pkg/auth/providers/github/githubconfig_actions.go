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
		return g.configTestApply(actionName, action, request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (g *GProvider) configureTest(actionName string, action *types.Action, request *types.APIContext) error {
	var githubConfig v3.GithubConfig
	githubConfigTestInput := v3.GithubConfigTestInput{}

	if err := json.NewDecoder(request.Request.Body).Decode(&githubConfigTestInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	githubConfig = githubConfigTestInput.GithubConfig
	redirectURL := formGithubRedirectURL(githubConfig)

	logrus.Debugf("redirecting the user to %v", redirectURL)
	http.Redirect(request.Response, request.Request, redirectURL, http.StatusFound)

	return nil

}

func formGithubRedirectURL(githubConfig v3.GithubConfig) string {
	redirect := ""
	if githubConfig.Hostname != "" {
		redirect = githubConfig.Scheme + githubConfig.Hostname
	} else {
		redirect = githubDefaultHostName
	}
	redirect = redirect + "/login/oauth/authorize?client_id=" + githubConfig.ClientID + "&scope=read:org"

	return redirect
}

func (g *GProvider) configTestApply(actionName string, action *types.Action, request *types.APIContext) error {
	var githubConfig v3.GithubConfig
	githubConfigApplyInput := v3.GithubConfigApplyInput{}

	if err := json.NewDecoder(request.Request.Body).Decode(&githubConfigApplyInput); err != nil {
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

	//TODO: create new user or update existing local User with github principals and use that userID in the token below.

	//create a new token, set this token as the cookie and return 200
	token := tokens.GenerateNewLoginToken(userPrincipal, groupPrincipals, providerInfo, 0, "Token via Github Configuration")
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
