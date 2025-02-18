package activedirectory

import (
	"fmt"
	"strings"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/retry"
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
		return p.testAndApply(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (p *adProvider) testAndApply(request *types.APIContext) error {
	input, err := handler.ParseAndValidateActionBody(request, request.Schemas.Schema(&managementschema.Version,
		client.ActiveDirectoryTestAndApplyInputType))
	if err != nil {
		return err
	}
	configApplyInput := &v32.ActiveDirectoryTestAndApplyInput{}
	if err := common.Decode(input, configApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	config := &configApplyInput.ActiveDirectoryConfig

	login := &v32.BasicLogin{
		Username: configApplyInput.Username,
		Password: configApplyInput.Password,
	}

	if config.ServiceAccountPassword != "" {
		value, err := common.ReadFromSecret(p.secrets, config.ServiceAccountPassword,
			strings.ToLower(client.ActiveDirectoryConfigFieldServiceAccountPassword))
		if err != nil {
			return err
		}
		config.ServiceAccountPassword = value
	}

	caPool, err := newCAPool(config.Certificate)
	if err != nil {
		return err
	}

	if len(config.Servers) < 1 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "must supply a server")
	}

	if config.UserSearchAttribute != "" {
		for _, attr := range strings.Split(config.UserSearchAttribute, "|") {
			if !ldap.IsValidAttr(attr) {
				return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid userSearchAttribute")
			}
		}
	}
	if config.UserLoginAttribute != "" && !ldap.IsValidAttr(config.UserLoginAttribute) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid userLoginAttribute")
	}
	if config.UserObjectClass != "" && !ldap.IsValidAttr(config.UserObjectClass) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid userObjectClass")
	}
	if config.UserNameAttribute != "" && !ldap.IsValidAttr(config.UserNameAttribute) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid userNameAttribute")
	}
	if config.UserEnabledAttribute != "" && !ldap.IsValidAttr(config.UserEnabledAttribute) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid userEnabledAttribute")
	}
	if config.GroupSearchAttribute != "" && !ldap.IsValidAttr(config.GroupSearchAttribute) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid groupSearchAttribute")
	}
	if config.GroupObjectClass != "" && !ldap.IsValidAttr(config.GroupObjectClass) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid groupObjectClass")
	}
	if config.GroupNameAttribute != "" && !ldap.IsValidAttr(config.GroupNameAttribute) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid groupNameAttribute")
	}
	if config.GroupDNAttribute != "" && !ldap.IsValidAttr(config.GroupDNAttribute) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid groupDNAttribute")
	}
	if config.GroupMemberUserAttribute != "" && !ldap.IsValidAttr(config.GroupMemberUserAttribute) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid groupMemberUserAttribute")
	}
	if config.GroupMemberMappingAttribute != "" && !ldap.IsValidAttr(config.GroupMemberMappingAttribute) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "invalid groupMemberMappingAttribute")
	}

	if config.UserLoginFilter != "" {
		if _, err := ldapv3.CompileFilter(config.UserLoginFilter); err != nil {
			return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "invalid userLoginFilter")
		}
	}
	if config.UserSearchFilter != "" {
		if _, err := ldapv3.CompileFilter(config.UserSearchFilter); err != nil {
			return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "invalid userSearchFilter")
		}
	}
	if config.GroupSearchFilter != "" {
		if _, err := ldapv3.CompileFilter(config.GroupSearchFilter); err != nil {
			return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "invalid groupSearchFilter")
		}
	}

	lConn, err := p.ldapConnection(config, caPool)
	if err != nil {
		return err
	}
	defer lConn.Close()

	userPrincipal, groupPrincipals, err := p.loginUser(lConn, login, config)
	if err != nil {
		return err
	}

	// If this works, save adConfig CR adding enabled flag.
	config.Enabled = configApplyInput.Enabled
	err = p.saveActiveDirectoryConfig(config)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to save activedirectory config: %v", err))
	}

	user, err := p.userMGR.SetPrincipalOnCurrentUser(request, userPrincipal)
	if err != nil {
		return err
	}

	userExtraInfo := p.GetUserExtraAttributes(userPrincipal)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return p.tokenMGR.UserAttributeCreateOrUpdate(user.Name, userPrincipal.Provider, groupPrincipals, userExtraInfo)
	}); err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Failed to create or update userAttribute: %v", err))
	}

	return p.tokenMGR.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, "", 0, "Token via AD Configuration", request)
}

func (p *adProvider) saveActiveDirectoryConfig(config *v32.ActiveDirectoryConfig) error {
	storedConfig, _, err := p.getActiveDirectoryConfig()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = client.ActiveDirectoryConfigType
	config.ObjectMeta = storedConfig.ObjectMeta

	field := strings.ToLower(client.ActiveDirectoryConfigFieldServiceAccountPassword)
	name, err := common.CreateOrUpdateSecrets(p.secrets, config.ServiceAccountPassword, field, strings.ToLower(convert.ToString(config.Type)))
	if err != nil {
		return err
	}

	config.ServiceAccountPassword = name

	logrus.Debugf("updating activeDirectoryConfig")
	_, err = p.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}
