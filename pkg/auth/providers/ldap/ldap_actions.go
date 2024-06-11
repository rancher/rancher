package ldap

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
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
		return p.testAndApply(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (p *ldapProvider) testAndApply(request *types.APIContext) error {
	var input map[string]interface{}
	var err error
	input, err = handler.ParseAndValidateActionBody(request, request.Schemas.Schema(&managementschema.Version,
		p.testAndApplyInputType))

	if err != nil {
		return err
	}

	configApplyInput := &v32.LdapTestAndApplyInput{}

	if err := common.Decode(input, configApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	config := &configApplyInput.LdapConfig

	login := &v32.BasicLogin{
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

	lConn, err := ldap.Connect(config, caPool)
	if err != nil {
		return err
	}
	defer lConn.Close()

	userPrincipal, groupPrincipals, err := p.loginUser(lConn, login, config, caPool)
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

	userExtraInfo := p.GetUserExtraAttributes(userPrincipal)
	err = p.tokenMGR.UserAttributeCreateOrUpdate(user.Name, userPrincipal.Provider, groupPrincipals, userExtraInfo)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("testAndApply: Failed to create or update userAttribute: %v", err))
	}

	return p.tokenMGR.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, "", 0, "Token via LDAP Configuration", request)
}

func (p *ldapProvider) saveLDAPConfig(config *v3.LdapConfig) error {
	storedConfig, _, err := p.getLDAPConfig(p.authConfigs.ObjectClient().UnstructuredClient())
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

	config.ServiceAccountPassword = common.GetFullSecretName(config.Type, field)

	logrus.Debugf("updating %s config", p.providerName)
	_, err = p.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}
