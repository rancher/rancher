package activedirectory

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
)

func (p *adProvider) formatter(apiContext *types.APIContext, resource *types.RawResource) {
	common.AddCommonActions(apiContext, resource)
	resource.AddAction(apiContext, "testAndApply")
}

func (p *adProvider) actionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	handled, err := common.HandleCommonAction(actionName, action, request, Name, p.authConfigs)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	if actionName == "testAndApply" {
		return p.testAndApply(actionName, action, request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (p *adProvider) testAndApply(actionName string, action *types.Action, request *types.APIContext) error {
	input, err := handler.ParseAndValidateActionBody(request, request.Schemas.Schema(&managementschema.Version,
		client.ActiveDirectoryTestAndApplyInputType))
	if err != nil {
		return err
	}
	configApplyInput := &v3.ActiveDirectoryTestAndApplyInput{}
	if err := mapstructure.Decode(input, configApplyInput); err != nil {
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

	userPrincipal, groupPrincipals, err := p.loginUser(login, config, caPool, true)
	if err != nil {
		return err
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

	return p.tokenMGR.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, "", 0, "Token via AD Configuration", request)
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
