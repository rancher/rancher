package keycloakoidc

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
)

func (k *keyCloakOIDCProvider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = k.actionHandler
	schema.Formatter = k.Formatter
}

func (k *keyCloakOIDCProvider) actionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	handled, err := common.HandleCommonAction(actionName, action, request, k.Name, k.AuthConfigs)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	switch actionName {
	case "configureTest":
		return k.ConfigureTest(request)
	case "testAndApply":
		return k.TestAndApply(request)
	default:
		return httperror.NewAPIError(httperror.ActionNotAvailable, "")
	}
}

// TestAndApply validates the correctness of the Keycloak OIDC configuration
// provided in the request.
// If the verification succeeds, it creates a Token to access the provider.
func (k *keyCloakOIDCProvider) TestAndApply(request *types.APIContext) error {
	var oidcConfig apiv3.KeyCloakOIDCConfig
	oidcConfigApplyInput := &apiv3.KeyCloakOIDCApplyInput{}

	if err := json.NewDecoder(request.Request.Body).Decode(oidcConfigApplyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("[keycloak oidc] testAndApply: failed to parse body: %v", err))
	}

	oidcConfig = oidcConfigApplyInput.OIDCConfig
	if oidcConfigApplyInput.OIDCConfig.GroupSearchEnabled == nil {
		oidcConfig.GroupSearchEnabled = ptr.To(false)
	}

	oidcLogin := &apiv3.OIDCLogin{
		Code: oidcConfigApplyInput.Code,
	}

	if !validateScopes(oidcConfig.Scopes) {
		return fmt.Errorf("scopes are invalid: scopes must be space delimited and openid is a required scope. %s", oidcConfig.Scopes)
	}

	issuerURL, err := url.Parse(oidcConfig.Issuer)
	if err != nil {
		if httperror.IsAPIError(err) {
			return err
		}
		return errors.Wrap(err, "[keycloak oidc]: failed to parse the issuer URL while authenticating")
	}
	oidcConfig.Issuer = issuerURL.String()

	userPrincipal, groupPrincipals, providerToken, _, err := k.LoginUser(
		request.Response, request.Request,
		oidcLogin, &oidcConfig.OIDCConfig)
	if err != nil {
		if httperror.IsAPIError(err) {
			return err
		}
		return errors.Wrap(err, "[keycloak oidc]: server error while authenticating")
	}
	user, err := k.UserMGR.SetPrincipalOnCurrentUser(request.Request, userPrincipal)
	if err != nil {
		return err
	}

	err = k.saveKeyCloakOIDCConfig(&oidcConfig)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("[keycloak oidc]: failed to save oidc config: %v", err))
	}

	userExtraInfo := k.GetUserExtraAttributes(userPrincipal)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return k.UserMGR.UserAttributeCreateOrUpdate(user.Name, userPrincipal.Provider, groupPrincipals, userExtraInfo)
	}); err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("[keycloak oidc]: Failed to create or update userAttribute: %v", err))
	}

	return k.TokenMgr.CreateTokenAndSetCookie(user.Name, userPrincipal, groupPrincipals, providerToken, 0, "Token via OIDC Configuration", request)
}

func (k *keyCloakOIDCProvider) saveKeyCloakOIDCConfig(config *apiv3.KeyCloakOIDCConfig) error {
	storedOIDCConfig, err := k.GetConfig()
	if err != nil {
		return err
	}

	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = k.Type
	config.ObjectMeta = storedOIDCConfig.ObjectMeta

	if config.PrivateKey != "" {
		privateKeyField := strings.ToLower(client.KeyCloakOIDCConfigFieldPrivateKey)
		name, err := common.CreateOrUpdateSecrets(k.Secrets, config.PrivateKey, privateKeyField, strings.ToLower(config.Type))
		if err != nil {
			return err
		}
		config.PrivateKey = name
	}

	secretField := strings.ToLower(client.KeyCloakOIDCConfigFieldClientSecret)
	name, err := common.CreateOrUpdateSecrets(k.Secrets, convert.ToString(config.ClientSecret), secretField, strings.ToLower(config.Type))
	if err != nil {
		return err
	}
	config.ClientSecret = name

	if config.OpenLdapConfig.ServiceAccountPassword != "" {
		name, err := common.SavePasswordSecret(k.Secrets, config.OpenLdapConfig.ServiceAccountPassword, client.LdapConfigFieldServiceAccountPassword, config.Type)
		if err != nil {
			return err
		}
		config.OpenLdapConfig.ServiceAccountPassword = name
	}

	logrus.Debugf("[keycloak oidc] saveKeyCloakOIDCConfig: updating config")
	_, err = k.AuthConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	return err
}

func validateScopes(input string) bool {
	if strings.Contains(input, ",") {
		return false
	}
	values := strings.Fields(input)
	for _, value := range values {
		if value == "openid" {
			return true
		}
	}
	return false
}
