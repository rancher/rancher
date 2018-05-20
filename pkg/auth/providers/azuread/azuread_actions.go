package azuread

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
)

func (p *azureADProvider) formatter(apiContext *types.APIContext, resource *types.RawResource) {
	common.AddCommonActions(apiContext, resource)
	resource.AddAction(apiContext, "testAndApply")
}

func (p *azureADProvider) actionHandler(actionName string, action *types.Action, request *types.APIContext) error {
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

func (p *azureADProvider) testAndApply(actionName string, action *types.Action, request *types.APIContext) error {
	input, err := handler.ParseAndValidateActionBody(request, request.Schemas.Schema(&managementschema.Version,
		client.AzureADTestAndApplyInputType))
	if err != nil {
		return err
	}
	configApplyInput := &v3.AzureADTestAndApplyInput{}
	if err := mapstructure.Decode(input, configApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}
	logrus.Debugf("configApplyInput %v", configApplyInput)

	config := configApplyInput.AzureADConfig

	login := &v3public.BasicLogin{
		Username: configApplyInput.Username,
		Password: configApplyInput.Password,
	}

	userPrincipal, groupPrincipals, providerInfo, err := p.loginUser(login, &config, true)
	if err != nil {
		return err
	}

	config.Enabled = configApplyInput.Enabled
	err = p.saveAzureADConfig(&config)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to save AzureAD config: %v", err))
	}

	user, err := p.userMGR.SetPrincipalOnCurrentUser(request, userPrincipal)
	if err != nil {
		return err
	}

	return tokens.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerInfo, 0, "Token via AzureAD Configuration", request)
}

func (p *azureADProvider) saveAzureADConfig(config *v3.AzureADConfig) error {
	storedConfig, err := p.getAzureADConfig()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = client.AzureADConfigType
	config.ObjectMeta = storedConfig.ObjectMeta

	logrus.Debugf("updating AzureADConfig")
	_, err = p.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}
