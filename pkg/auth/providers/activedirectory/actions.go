package activedirectory

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
)

func (p *adProvider) formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "testAndApply")
}

func (p *adProvider) actionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == "testAndApply" {
		return p.testAndApply(actionName, action, request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (p *adProvider) testAndApply(actionName string, action *types.Action, request *types.APIContext) error {
	configApplyInput := &v3.ActiveDirectoryTestAndApplyInput{}
	if err := json.NewDecoder(request.Request.Body).Decode(configApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}
	logrus.Debugf("configApplyInput %v", configApplyInput)

	config := &configApplyInput.ActiveDirectoryConfig

	login := &v3public.BasicLogin{
		Username: configApplyInput.Username,
		Password: configApplyInput.Password,
	}

	caPool, err := newCAPool(config.Certificate)
	if err != nil {
		return err
	}

	if len(config.Servers) < 1 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "must supply a server")
	}
	if len(config.Servers) > 1 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "multiple servers not yet supported")
	}

	userPrincipal, groupPrincipals, providerInfo, status, err := p.loginUser(login, config, caPool)
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
	err = p.saveActiveDirectoryConfig(config)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to save activedirectory config: %v", err))
	}

	user, err := p.userMGR.SetPrincipalOnCurrentUser(request, userPrincipal)
	if err != nil {
		return err
	}

	return tokens.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerInfo, 0, "Token via AD Configuration", request)
}

func (p *adProvider) saveActiveDirectoryConfig(config *v3.ActiveDirectoryConfig) error {
	storedConfig, _, err := p.getActiveDirectoryConfig()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = client.ActiveDirectoryConfigType
	config.ObjectMeta = storedConfig.ObjectMeta

	logrus.Debugf("updating githubConfig")
	_, err = p.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}
