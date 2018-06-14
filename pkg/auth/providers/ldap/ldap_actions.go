package ldap

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

func (p *ldapProvider) formatter(apiContext *types.APIContext, resource *types.RawResource) {
	common.AddCommonActions(apiContext, resource)
	resource.AddAction(apiContext, "testAndApply")
}

func (p *ldapProvider) actionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	handled, err := common.HandleCommonAction(actionName, action, request, p.providerName, p.authConfigs)
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

func (p *ldapProvider) testAndApply(actionName string, action *types.Action, request *types.APIContext) error {
	var input map[string]interface{}
	var err error
	if p.providerName == "openldap" {
		input, err = handler.ParseAndValidateActionBody(request, request.Schemas.Schema(&managementschema.Version,
			client.OpenLdapTestAndApplyInputType))
	} else {
		input, err = handler.ParseAndValidateActionBody(request, request.Schemas.Schema(&managementschema.Version,
			client.FreeIpaTestAndApplyInputType))
	}

	if err != nil {
		return err
	}

	configApplyInput := &v3.LdapTestAndApplyInput{}

	if err := mapstructure.Decode(input, configApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}
	logrus.Debugf("configApplyInput %v", configApplyInput)

	config := &configApplyInput.LdapConfig

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

	userPrincipal, groupPrincipals, providerInfo, err := p.loginUser(login, config, caPool)
	if err != nil {
		return err
	}

	//if this works, save LDAPConfig CR adding enabled flag
	config.Enabled = configApplyInput.Enabled
	err = p.saveLDAPConfig(config)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to save %s config: %v", p.providerName, err))
	}

	user, err := p.userMGR.SetPrincipalOnCurrentUser(request, userPrincipal)
	if err != nil {
		return err
	}

	return tokens.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerInfo, 0, "Token via LDAP Configuration", request)
}

func (p *ldapProvider) saveLDAPConfig(config *v3.LdapConfig) error {
	storedConfig, _, err := p.getLDAPConfig()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	if p.providerName == "openldap" {
		config.Type = client.OpenLdapConfigType
	} else {
		config.Type = client.FreeIpaConfigType
	}

	config.ObjectMeta = storedConfig.ObjectMeta

	logrus.Debugf("updating %s config", p.providerName)
	_, err = p.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}
