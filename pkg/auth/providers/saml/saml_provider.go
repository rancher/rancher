package saml

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/crewjam/saml"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/api/secrets"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	PingName            = "ping"
	ADFSName            = "adfs"
	KeyCloakName        = "keycloak"
	OKTAName            = "okta"
	ShibbolethName      = "shibboleth"
	loginAction         = "login"
	testAndEnableAction = "testAndEnable"
)

type Provider struct {
	ctx             context.Context
	authConfigs     v3.AuthConfigInterface
	secrets         corev1.SecretInterface
	samlTokens      v3.SamlTokenInterface
	userMGR         user.Manager
	tokenMGR        *tokens.Manager
	serviceProvider *saml.ServiceProvider
	name            string
	userType        string
	groupType       string
	clientState     ClientState
	ldapProvider    common.AuthProvider
}

var SamlProviders = make(map[string]*Provider)

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager, name string) common.AuthProvider {
	samlp := &Provider{
		ctx:         ctx,
		authConfigs: mgmtCtx.Management.AuthConfigs(""),
		secrets:     mgmtCtx.Core.Secrets(""),
		samlTokens:  mgmtCtx.Management.SamlTokens(""),
		userMGR:     userMGR,
		tokenMGR:    tokenMGR,
		name:        name,
		userType:    name + "_user",
		groupType:   name + "_group",
	}

	if samlp.hasLdapGroupSearch() {
		samlp.ldapProvider = ldap.Configure(ctx, mgmtCtx, userMGR, tokenMGR, name)
	}

	SamlProviders[name] = samlp
	return samlp
}

func (s *Provider) GetName() string {
	return s.name
}

func (s *Provider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = s.actionHandler
	schema.Formatter = s.formatter
}

func (s *Provider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	p := common.TransformToAuthProvider(authConfig)
	switch s.name {
	case PingName:
		p[publicclient.PingProviderFieldRedirectURL] = formSamlRedirectURLFromMap(authConfig, s.name)
	case ADFSName:
		p[publicclient.ADFSProviderFieldRedirectURL] = formSamlRedirectURLFromMap(authConfig, s.name)
	case KeyCloakName:
		p[publicclient.KeyCloakProviderFieldRedirectURL] = formSamlRedirectURLFromMap(authConfig, s.name)
	case OKTAName:
		p[publicclient.OKTAProviderFieldRedirectURL] = formSamlRedirectURLFromMap(authConfig, s.name)
	case ShibbolethName:
		p[publicclient.ShibbolethProviderFieldRedirectURL] = formSamlRedirectURLFromMap(authConfig, s.name)
	}
	return p, nil
}

func (s *Provider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	return v3.Principal{}, nil, "", fmt.Errorf("SAML providers do not implement Authenticate User API")
}

func PerformSamlLogin(name string, apiContext *types.APIContext, input interface{}) error {
	//input will contain the FINAL redirect URL
	login, ok := input.(*v32.SamlLoginInput)
	if !ok {
		return errors.New("unexpected input type")
	}
	finalRedirectURL := login.FinalRedirectURL

	if provider, ok := SamlProviders[name]; ok {
		if provider == nil {
			logrus.Errorf("SAML: Provider %v not initialized", name)
			return fmt.Errorf("SAML: Provider %v not initialized", name)
		}
		if provider.clientState == nil {
			logrus.Errorf("SAML: Provider %v clientState not set", name)
			return fmt.Errorf("SAML: Provider %v clientState not set", name)
		}
		logrus.Debugf("SAML [PerformSamlLogin]: Setting clientState for SAML service provider %v", name)
		provider.clientState.SetState(apiContext.Response, apiContext.Request, "Rancher_FinalRedirectURL", finalRedirectURL)
		provider.clientState.SetState(apiContext.Response, apiContext.Request, "Rancher_Action", loginAction)
		provider.clientState.SetState(apiContext.Response, apiContext.Request, "Rancher_PublicKey", login.PublicKey)
		provider.clientState.SetState(apiContext.Response, apiContext.Request, "Rancher_RequestID", login.RequestID)
		provider.clientState.SetState(apiContext.Response, apiContext.Request, "Rancher_ResponseType", login.ResponseType)

		idpRedirectURL, err := provider.HandleSamlLogin(apiContext.Response, apiContext.Request)
		if err != nil {
			return err
		}
		logrus.Debugf("SAML [PerformSamlLogin]: Redirecting to the identity provider login page at %v", idpRedirectURL)
		data := map[string]interface{}{
			"idpRedirectUrl": idpRedirectURL,
			"type":           "samlLoginOutput",
		}
		apiContext.WriteResponse(http.StatusOK, data)

		return nil
	}
	return nil
}

func (s *Provider) getSamlConfig() (*v32.SamlConfig, error) {
	authConfigObj, err := s.authConfigs.ObjectClient().UnstructuredClient().Get(s.name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("SAML: failed to retrieve SamlConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("SAML: failed to retrieve SamlConfig, cannot read k8s Unstructured data")
	}
	storedSamlConfigMap := u.UnstructuredContent()

	storedSamlConfig := &v32.SamlConfig{}
	err = common.Decode(storedSamlConfigMap, storedSamlConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode Saml Config: %w", err)
	}

	if enabled, ok := storedSamlConfigMap["enabled"].(bool); ok {
		storedSamlConfig.Enabled = enabled
	}

	if storedSamlConfig.SpKey != "" {
		value, err := common.ReadFromSecret(s.secrets, storedSamlConfig.SpKey,
			strings.ToLower(client.PingConfigFieldSpKey))
		if err != nil {
			return nil, err
		}
		storedSamlConfig.SpKey = value
	}

	return storedSamlConfig, nil
}

func (s *Provider) saveSamlConfig(config *v32.SamlConfig) error {
	var configType string

	storedSamlConfig, err := s.getSamlConfig()
	if err != nil {
		return err
	}

	switch s.name {
	case PingName:
		configType = client.PingConfigType
	case ADFSName:
		configType = client.ADFSConfigType
	case KeyCloakName:
		configType = client.KeyCloakConfigType
	case OKTAName:
		configType = client.OKTAConfigType
	case ShibbolethName:
		configType = client.ShibbolethConfigType
	}

	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = configType
	storedSamlConfig.Annotations = config.Annotations
	config.ObjectMeta = storedSamlConfig.ObjectMeta

	var field string
	// This assumes the provider needs to create only one secret. If there are new entries
	// in the secret collection, this code that creates the actual secrets would need to be updated.
	if fields, ok := secrets.TypeToFields[configType]; ok && len(fields) > 0 {
		field = strings.ToLower(fields[0])
	}
	if err := common.CreateOrUpdateSecrets(s.secrets, config.SpKey,
		field, strings.ToLower(config.Type)); err != nil {
		return err
	}

	config.SpKey = common.GetFullSecretName(config.Type, field)
	if s.hasLdapGroupSearch() {
		combinedConfig, err := s.combineSamlAndLdapConfig(config)
		if err != nil {
			logrus.Warnf("problem combining saml and ldap config, saving partial configuration %s", err.Error())
		}
		_, err = s.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, combinedConfig)
		if err != nil {
			return fmt.Errorf("unable to update authconfig: %w", err)
		}
		return nil
	}

	_, err = s.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	return err
}

func (s *Provider) toPrincipal(principalType string, princ v3.Principal, token *v3.Token) v3.Principal {
	if principalType == s.userType {
		princ.PrincipalType = "user"
		if token != nil {
			princ.Me = s.isThisUserMe(token.UserPrincipal, princ)
			if princ.Me {
				princ.LoginName = token.UserPrincipal.LoginName
				princ.DisplayName = token.UserPrincipal.DisplayName
			}
		}
	} else {
		princ.PrincipalType = "group"
		if token != nil {
			princ.MemberOf = s.tokenMGR.IsMemberOf(*token, princ)
		}
	}

	return princ
}

func (s *Provider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	return nil, errors.New("Not implemented")
}

func (s *Provider) SearchPrincipals(searchKey, principalType string, token v3.Token) ([]v3.Principal, error) {
	if s.hasLdapGroupSearch() {
		principals, err := s.ldapProvider.SearchPrincipals(searchKey, principalType, token)
		// only give response from ldap if it's configured
		if !ldap.IsNotConfigured(err) {
			return principals, err
		}
	}

	var principals []v3.Principal
	if principalType == "" {
		principalType = "user"
	}

	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: s.userType + "://" + searchKey},
		DisplayName:   searchKey,
		LoginName:     searchKey,
		PrincipalType: principalType,
		Provider:      s.name,
	}

	principals = append(principals, p)
	return principals, nil
}

func (s *Provider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	externalID, principalType := splitPrincipalID(principalID)
	if externalID == "" && principalType == "" {
		return v3.Principal{}, fmt.Errorf("SAML: invalid id %v", principalID)
	}
	if principalType != s.userType && principalType != s.groupType {
		return v3.Principal{}, fmt.Errorf("SAML: Invalid principal type")
	}

	if s.hasLdapGroupSearch() {
		p, err := s.ldapProvider.GetPrincipal(principalID, token)
		// only give response from ldap if it's configured
		if !ldap.IsNotConfigured(err) {
			return p, err
		}
	}

	p := v3.Principal{
		ObjectMeta:  metav1.ObjectMeta{Name: principalType + "://" + externalID},
		DisplayName: externalID,
		LoginName:   externalID,
		Provider:    s.name,
	}

	p = s.toPrincipal(principalType, p, &token)
	return p, nil
}

func (s *Provider) isThisUserMe(me v3.Principal, other v3.Principal) bool {
	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}

func (s *Provider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []v3.Principal) (bool, error) {
	config, err := s.getSamlConfig()
	if err != nil {
		logrus.Errorf("Error fetching saml config: %v", err)
		return false, err
	}
	allowed, err := s.userMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipalID, groupPrincipals)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func formSamlRedirectURLFromMap(config map[string]interface{}, name string) string {
	var hostname string
	switch name {
	case PingName:
		hostname, _ = config[client.PingConfigFieldRancherAPIHost].(string)
	case ADFSName:
		hostname, _ = config[client.ADFSConfigFieldRancherAPIHost].(string)
	case KeyCloakName:
		hostname, _ = config[client.KeyCloakConfigFieldRancherAPIHost].(string)
	case OKTAName:
		hostname, _ = config[client.OKTAConfigFieldRancherAPIHost].(string)
	case ShibbolethName:
		hostname, _ = config[client.ShibbolethConfigFieldRancherAPIHost].(string)
	}

	path := hostname + "/v1-saml/" + name + "/login"
	return path
}

func splitPrincipalID(principalID string) (string, string) {
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	externalID := strings.TrimPrefix(parts[1], "//")
	return externalID, parts[0]
}

func (s *Provider) combineSamlAndLdapConfig(config *v32.SamlConfig) (runtime.Object, error) {
	// if errors we might not want to turn on ldap
	ldapConfig, _, err := ldap.GetLDAPConfig(s.ldapProvider)

	// can be misconfigured but still want it saved
	if err != nil {
		logrus.Warnf("error pulling %s ldap configs: %s\n", s.name, err)

		// if the the config subkey not in the crd
		if ldapConfig == nil {
			return config, nil
		}

		// only return the saml config on other errors
		// if not configured it might have data in it we want to keep
		if !ldap.IsNotConfigured(err) {
			return config, nil
		}
	}

	var fullConfig runtime.Object
	samlConfig := v32.SamlConfig{}
	config.DeepCopyInto(&samlConfig)
	switch s.name {
	case ShibbolethName:
		secretName, err := common.SavePasswordSecret(
			s.secrets,
			ldapConfig.LdapFields.ServiceAccountPassword,
			client.LdapConfigFieldServiceAccountPassword,
			samlConfig.Type,
		)
		if err != nil {
			return config, fmt.Errorf("unable to save ldap service account password: %w", err)
		}

		ldapConfig.LdapFields.ServiceAccountPassword = secretName
		// Set the status for SecretsMigrated to True so it doesn't get re-migrated
		v32.AuthConfigConditionSecretsMigrated.SetStatus(&samlConfig, "True")
		fullConfig = &v32.ShibbolethConfig{
			SamlConfig:     samlConfig,
			OpenLdapConfig: ldapConfig.LdapFields,
		}
	case OKTAName:
		fullConfig = &v32.OKTAConfig{
			SamlConfig:     samlConfig,
			OpenLdapConfig: ldapConfig.LdapFields,
		}
	}

	return fullConfig, nil
}

func (s *Provider) hasLdapGroupSearch() bool {
	return ShibbolethName == s.name || OKTAName == s.name
}

func (s *Provider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	extras := make(map[string][]string)
	if userPrincipal.Name != "" {
		extras[common.UserAttributePrincipalID] = []string{userPrincipal.Name}
	}
	if userPrincipal.LoginName != "" {
		extras[common.UserAttributeUserName] = []string{userPrincipal.LoginName}
	}
	return extras
}

// IsDisabledProvider checks if the SAML auth provider is currently disabled in Rancher.
func (s *Provider) IsDisabledProvider() (bool, error) {
	samlConfig, err := s.getSamlConfig()
	if err != nil {
		return false, err
	}
	return !samlConfig.Enabled, nil
}
