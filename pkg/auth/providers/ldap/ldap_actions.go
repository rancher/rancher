package ldap

import (
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	client "github.com/rancher/types/client/management/v3"
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
	input, err = handler.ParseAndValidateActionBody(request, request.Schemas.Schema(&managementschema.Version,
		p.testAndApplyInputType))

	if err != nil {
		return err
	}

	configApplyInput := &v3.LdapTestAndApplyInput{}

	if err := mapstructure.Decode(input, configApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	config := &configApplyInput.LdapConfig

	login := &v3public.BasicLogin{
		Username: configApplyInput.Username,
		Password: configApplyInput.Password,
	}

	if config.ServiceAccountPassword != "" {
		value, err := common.ReadFromSecret(p.secrets, config.ServiceAccountPassword,
			strings.ToLower(client.LdapConfigFieldServiceAccountPassword))
		if err != nil {
			return err
		}
		config.ServiceAccountPassword = value
	}

	caPool, err := ldap.NewCAPool(config.Certificate)
	if err != nil {
		return err
	}

	if len(config.Servers) < 1 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "must supply a server")
	}
	if len(config.Servers) > 1 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "multiple servers not yet supported")
	}
	userPrincipal, groupPrincipals, err := p.loginUser(login, config, caPool)
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
	return p.tokenMGR.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, "", 0, "Token via LDAP Configuration", request)
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

	field := strings.ToLower(client.LdapConfigFieldServiceAccountPassword)
	if err := common.CreateOrUpdateSecrets(p.secrets, config.ServiceAccountPassword,
		field, strings.ToLower(config.Type)); err != nil {
		return err
	}

	config.ServiceAccountPassword = common.GetName(config.Type, field)

	logrus.Debugf("updating %s config", p.providerName)
	_, err = p.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}
