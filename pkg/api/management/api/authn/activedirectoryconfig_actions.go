package authn

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"

	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/ldap/activedirectory"
	"github.com/rancher/rancher/pkg/auth/tokens"
)

func ActiveDirectoryConfigFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "testAndApply")
}

func ActiveDirectoryConfigActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == "testAndApply" {
		return ActiveDirectoryConfigTestApply(actionName, action, request)
	}

	return nil
}

func ActiveDirectoryConfigTestApply(actionName string, action *types.Action, request *types.APIContext) error {
	var config v3.ActiveDirectoryConfig
	var activeDirectoryCredential v3.ActiveDirectoryCredential
	configApplyInput := v3.ActiveDirectoryConfigApplyInput{}

	if err := json.NewDecoder(request.Request.Body).Decode(&configApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	logrus.Debugf("configApplyInput %v", configApplyInput)

	config = configApplyInput.ActiveDirectoryConfig
	activeDirectoryCredential = configApplyInput.ActiveDirectoryCredential

	//Call provider to testLogin
	p, err := providers.GetProvider("activedirectory")
	if err != nil {
		return err
	}
	adProvider, ok := p.(*activedirectory.AProvider)
	if !ok {
		return fmt.Errorf("No activedirectory provider")
	}

	userPrincipal, groupPrincipals, providerInfo, status, err := adProvider.LoginUser(activeDirectoryCredential, &config)
	if err != nil {
		if status == 0 || status == 500 {
			status = http.StatusInternalServerError
			return httperror.NewAPIErrorLong(status, "ServerError", fmt.Sprintf("Failed to login to activedirectory: %v", err))
		}
		return httperror.NewAPIErrorLong(status, "",
			fmt.Sprintf("Failed to login to ActiveDirectory: %v", err))
	}

	//if this works, save adConfig CR adding enabled flag
	config.Enabled = configApplyInput.Enabled
	err = adProvider.SaveActiveDirectoryConfig(&config)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to save activedirectory config: %v", err))
	}

	//TODO: create new user or update exiting local User with ad principals and use that userID in the token below.

	//create a new token, set this token as the cookie and return 200
	token := tokens.GenerateNewLoginToken(userPrincipal, groupPrincipals, providerInfo, 0, "Token via ActiveDirectory Configuration")
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
