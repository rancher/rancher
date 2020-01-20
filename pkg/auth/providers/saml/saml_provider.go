package saml

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/store/auth"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	ldaputils "github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"github.com/rancher/rancher/pkg/auth/tokens"
	corev1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	client "github.com/rancher/types/client/management/v3"
	publicclient "github.com/rancher/types/client/management/v3public"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
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
	ctx                context.Context
	authConfigs        v3.AuthConfigInterface
	secrets            corev1.SecretInterface
	userMGR            user.Manager
	tokenMGR           *tokens.Manager
	serviceProvider    *saml.ServiceProvider
	name               string
	userType           string
	groupType          string
	certs              string
	caPool             *x509.CertPool
	clientState        samlsp.ClientState
	ldapProvider       common.AuthProvider
	hasValidLdapConfig bool
}

var SamlProviders = make(map[string]*Provider)

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager, name string) common.AuthProvider {
	samlp := &Provider{
		ctx:         ctx,
		authConfigs: mgmtCtx.Management.AuthConfigs(""),
		secrets:     mgmtCtx.Core.Secrets(""),
		userMGR:     userMGR,
		tokenMGR:    tokenMGR,
		name:        name,
		userType:    name + "_user",
		groupType:   name + "_group",
	}

	if samlp.hasLdapGroupSearch() {
		samlp.ldapProvider = ldap.Configure(ctx, mgmtCtx, userMGR, tokenMGR, name)

		// ignoring errors during boot if not configured
		ldapConf, certpool, err := samlp.getLDAPConfig()
		if err != nil {
			logrus.Infof("getting %s ldap config failed: %s", samlp.name, err)
		}
		samlp.hasValidLdapConfig, err = ldaputils.ValidateLdapConfig(ldapConf, certpool)
		if err != nil {
			logrus.Infof("validating %s ldap config failed: %s", samlp.name, err)
		}
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
	login, ok := input.(*v3public.SamlLoginInput)
	if !ok {
		return errors.New("unexpected input type")
	}
	finalRedirectURL := login.FinalRedirectURL

	if provider, ok := SamlProviders[name]; ok {
		provider.clientState.SetState(apiContext.Response, apiContext.Request, "Rancher_FinalRedirectURL", finalRedirectURL)
		provider.clientState.SetState(apiContext.Response, apiContext.Request, "Rancher_Action", loginAction)
		idpRedirectURL, err := provider.HandleSamlLogin(apiContext.Response, apiContext.Request)
		if err != nil {
			return err
		}
		data := map[string]interface{}{
			"idpRedirectUrl": idpRedirectURL,
			"type":           "samlLoginOutput",
		}

		apiContext.WriteResponse(http.StatusOK, data)
		return nil
	}

	return nil
}

func (s *Provider) getLDAPConfig() (*v3.LdapConfig, *x509.CertPool, error) {
	// TODO See if this can be simplified. also, this makes an api call everytime. find a better way
	authConfigObj, err := s.authConfigs.ObjectClient().UnstructuredClient().Get(s.name, metav1.GetOptions{})

	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve %s, error: %v", s.name, err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, nil, fmt.Errorf("failed to retrieve %s, cannot read k8s Unstructured data", s.name)
	}
	storedLdapConfigMap := u.UnstructuredContent()
	storedLdapConfig := &v3.LdapConfig{}
	mapstructure.Decode(storedLdapConfigMap, storedLdapConfig)

	metadataMap, ok := storedLdapConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("failed to retrieve %s metadata, cannot read k8s Unstructured data", s.name)
	}

	objectMeta := &metav1.ObjectMeta{}
	mapstructure.Decode(metadataMap, objectMeta)
	storedLdapConfig.ObjectMeta = *objectMeta

	if s.certs != storedLdapConfig.Certificate || s.caPool == nil {
		pool, err := ldaputils.NewCAPool(storedLdapConfig.Certificate)
		if err != nil {
			return nil, nil, err
		}
		s.certs = storedLdapConfig.Certificate
		s.caPool = pool
	}

	if storedLdapConfig.ServiceAccountPassword != "" {
		value, err := common.ReadFromSecret(s.secrets, storedLdapConfig.ServiceAccountPassword,
			strings.ToLower(client.LdapConfigFieldServiceAccountPassword))
		if err != nil {
			return nil, nil, err
		}
		storedLdapConfig.ServiceAccountPassword = value
	}

	return storedLdapConfig, s.caPool, nil
}

func (s *Provider) getSamlConfig() (*v3.SamlConfig, error) {
	authConfigObj, err := s.authConfigs.ObjectClient().UnstructuredClient().Get(s.name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("SAML: failed to retrieve SamlConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("SAML: failed to retrieve SamlConfig, cannot read k8s Unstructured data")
	}
	storedSamlConfigMap := u.UnstructuredContent()

	storedSamlConfig := &v3.SamlConfig{}
	mapstructure.Decode(storedSamlConfigMap, storedSamlConfig)

	if enabled, ok := storedSamlConfigMap["enabled"].(bool); ok {
		storedSamlConfig.Enabled = enabled
	}

	metadataMap, ok := storedSamlConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("SAML: failed to retrieve SamlConfig metadata, cannot read k8s Unstructured data")
	}

	objectMeta := &metav1.ObjectMeta{}
	mapstructure.Decode(metadataMap, objectMeta)
	storedSamlConfig.ObjectMeta = *objectMeta

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

func (s *Provider) saveSamlConfig(config *v3.SamlConfig) error {
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

	field := strings.ToLower(auth.TypeToFields[configType][0])
	if err := common.CreateOrUpdateSecrets(s.secrets, config.SpKey,
		field, strings.ToLower(config.Type)); err != nil {
		return err
	}

	config.SpKey = common.GetName(config.Type, field)
	if s.hasLdapGroupSearch() {
		combinedConfig, err := s.combineSamlAndLdapConfig(config)
		if err != nil {
			return err
		}

		_, err = s.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, combinedConfig)
		return err
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
	if s.hasValidLdapGroupSearch() {
		externalID, principalType := splitPrincipalID(principalID)
		if principalType == "user" {
			userPrincipals, err := s.ldapProvider.SearchPrincipals(externalID, "user", v3.Token{})
			if err != nil || len(userPrincipals) != 1 {
				return nil, err
			}

			return s.ldapProvider.RefetchGroupPrincipals(userPrincipals[0].Name, secret)
		}
	}

	return nil, errors.New("Not implemented")
}

func (s *Provider) SearchPrincipals(searchKey, principalType string, token v3.Token) ([]v3.Principal, error) {
	if s.hasValidLdapGroupSearch() {
		return s.ldapProvider.SearchPrincipals(searchKey, principalType, token)
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

	logrus.Debugf("GET PRINCIPAL %s \n", principalID)
	if s.hasValidLdapGroupSearch() {
		return s.ldapProvider.GetPrincipal(principalID, token)

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

func (s *Provider) combineSamlAndLdapConfig(config *v3.SamlConfig) (runtime.Object, error) {
	samlConfig := v3.SamlConfig{}
	config.DeepCopyInto(&samlConfig)

	ldapConfig, certpool, err := s.getLDAPConfig()
	if err != nil {
		return nil, err
	}

	s.hasValidLdapConfig, err = ldaputils.ValidateLdapConfig(ldapConfig, certpool)
	if err != nil {
		logrus.Warnf("could not validate %s ldap configs: %s\n", s.name, err)
	}

	var fullConfig runtime.Object
	switch s.name {
	case ShibbolethName:
		fullConfig = &v3.ShibbolethConfig{
			SamlConfig: samlConfig,
			LdapFields: ldapConfig.LdapFields,
		}
	}

	return fullConfig, nil
}

func (s *Provider) hasLdapGroupSearch() bool {
	if s.name == ShibbolethName {
		return true
	}
	return false
}

func (s *Provider) hasValidLdapGroupSearch() bool {
	if s.ldapProvider != nil && s.hasValidLdapConfig {
		return true
	}

	return false
}
