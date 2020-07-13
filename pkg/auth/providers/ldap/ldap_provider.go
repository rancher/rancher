package ldap

import (
	"context"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	"github.com/rancher/rancher/pkg/auth/tokens"
	corev1 "github.com/rancher/rancher/pkg/types/apis/core/v1"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3public"
	client "github.com/rancher/rancher/pkg/types/client/management/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/user"
	"github.com/sirupsen/logrus"
	ldapv2 "gopkg.in/ldap.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	OpenLdapName   = "openldap"
	FreeIpaName    = "freeipa"
	ShibbolethName = "shibboleth"
	ObjectClass    = "objectClass"
)

var (
	errNotConfigured       = fmt.Errorf("not configured")
	testAndApplyInputTypes = map[string]string{
		FreeIpaName:  client.FreeIpaTestAndApplyInputType,
		OpenLdapName: client.OpenLdapTestAndApplyInputType,
	}

	// empty string for inline
	ldapConfigKey = map[string]string{
		FreeIpaName:    "",
		OpenLdapName:   "",
		ShibbolethName: client.ShibbolethConfigFieldOpenLdapConfig,
	}
)

type ldapProvider struct {
	ctx                   context.Context
	authConfigs           v3.AuthConfigInterface
	secrets               corev1.SecretInterface
	userMGR               user.Manager
	tokenMGR              *tokens.Manager
	certs                 string
	caPool                *x509.CertPool
	providerName          string
	testAndApplyInputType string
	userScope             string
	groupScope            string
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager, providerName string) common.AuthProvider {
	return &ldapProvider{
		ctx:                   ctx,
		authConfigs:           mgmtCtx.Management.AuthConfigs(""),
		secrets:               mgmtCtx.Core.Secrets(""),
		userMGR:               userMGR,
		tokenMGR:              tokenMGR,
		providerName:          providerName,
		testAndApplyInputType: testAndApplyInputTypes[providerName],
		userScope:             providerName + "_user",
		groupScope:            providerName + "_group",
	}
}

func GetLDAPConfig(authProvider common.AuthProvider) (*v3.LdapConfig, *x509.CertPool, error) {
	ldapProvider, ok := authProvider.(*ldapProvider)
	if !ok {
		return nil, nil, fmt.Errorf("can not get ldap config from type other than ldapProvider")
	}

	return ldapProvider.getLDAPConfig()
}

func IsNotConfigured(err error) bool {
	return err == errNotConfigured
}

func (p *ldapProvider) GetName() string {
	return p.providerName
}

func (p *ldapProvider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = p.actionHandler
	schema.Formatter = p.formatter
}

func (p *ldapProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	ldap := common.TransformToAuthProvider(authConfig)
	return ldap, nil
}

func (p *ldapProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	login, ok := input.(*v3public.BasicLogin)
	if !ok {
		return v3.Principal{}, nil, "", errors.New("unexpected input type")
	}

	config, caPool, err := p.getLDAPConfig()
	if err != nil {
		return v3.Principal{}, nil, "", errors.New("can't find authprovider")
	}

	principal, groupPrincipal, err := p.loginUser(login, config, caPool)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	return principal, groupPrincipal, "", err
}

// searchKey can be user PrincipalID e.g. shibboleth_user://username with principalType of group for group search by user
func (p *ldapProvider) SearchPrincipals(searchKey, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal
	var err error

	config, caPool, err := p.getLDAPConfig()
	if err != nil {
		if IsNotConfigured(err) {
			return principals, err
		}
		logrus.Warnf("ldap search principals failed to get ldap config: %s\n", err)
		return principals, nil
	}

	lConn, err := ldap.Connect(config, caPool)
	if err != nil {
		logrus.Warnf("ldap search principals failed to connect to ldap: %s\n", err)
		return principals, nil
	}
	defer lConn.Close()

	principals, err = p.searchPrincipals(searchKey, principalType, config, lConn)
	if err == nil {
		for _, principal := range principals {
			if principal.PrincipalType == "user" {
				if p.isThisUserMe(myToken.UserPrincipal, principal) {
					principal.Me = true
				}
			} else if principal.PrincipalType == "group" {
				if p.isMemberOf(myToken.GroupPrincipals, principal) {
					principal.MemberOf = true
				}
			}
		}
	}

	return principals, nil
}

func (p *ldapProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	config, caPool, err := p.getLDAPConfig()
	if err != nil {
		if IsNotConfigured(err) {
			return v3.Principal{}, err
		}
		return v3.Principal{}, nil
	}

	externalID, scope, err := p.getDNAndScopeFromPrincipalID(principalID)
	if err != nil {
		return v3.Principal{}, err
	}

	var principal *v3.Principal
	if p.samlSearchProvider() {
		principal, err = p.samlSearchGetPrincipal(externalID, scope, config, caPool)
	} else {
		principal, err = p.getPrincipal(externalID, scope, config, caPool)
	}

	if err != nil {
		return v3.Principal{}, err
	}

	if p.isThisUserMe(token.UserPrincipal, *principal) {
		principal.Me = true
	}
	return *principal, err
}

func (p *ldapProvider) isThisUserMe(me v3.Principal, other v3.Principal) bool {
	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}

func (p *ldapProvider) isMemberOf(myGroups []v3.Principal, other v3.Principal) bool {
	for _, mygroup := range myGroups {
		if mygroup.ObjectMeta.Name == other.ObjectMeta.Name && mygroup.PrincipalType == other.PrincipalType {
			return true
		}
	}
	return false
}

func (p *ldapProvider) getLDAPConfig() (*v3.LdapConfig, *x509.CertPool, error) {
	// TODO See if this can be simplified. also, this makes an api call everytime. find a better way
	authConfigObj, err := p.authConfigs.ObjectClient().UnstructuredClient().Get(p.providerName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve %s, error: %v", p.providerName, err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, nil, fmt.Errorf("failed to retrieve %s, cannot read k8s Unstructured data", p.providerName)
	}

	storedLdapConfigMap := u.UnstructuredContent()
	storedLdapConfig := &v3.LdapConfig{}

	if p.samlSearchProvider() && ldapConfigKey[p.providerName] != "" {
		subLdapConfig, ok := storedLdapConfigMap[ldapConfigKey[p.providerName]]
		if !ok {
			return nil, nil, errNotConfigured
		}

		storedLdapConfigMap = subLdapConfig.(map[string]interface{})
		mapstructure.Decode(storedLdapConfigMap, storedLdapConfig)
		if len(storedLdapConfig.Servers) != 1 {
			return storedLdapConfig, nil, errNotConfigured
		}
	} else {
		mapstructure.Decode(storedLdapConfigMap, storedLdapConfig)
		metadataMap, ok := storedLdapConfigMap["metadata"].(map[string]interface{})
		if !ok {
			return nil, nil, fmt.Errorf("failed to retrieve %s metadata, cannot read k8s Unstructured data", p.providerName)
		}
		objectMeta := &metav1.ObjectMeta{}
		mapstructure.Decode(metadataMap, objectMeta)
		storedLdapConfig.ObjectMeta = *objectMeta
	}

	if p.certs != storedLdapConfig.Certificate || p.caPool == nil {
		pool, err := ldap.NewCAPool(storedLdapConfig.Certificate)
		if err != nil {
			return nil, nil, err
		}
		p.certs = storedLdapConfig.Certificate
		p.caPool = pool
	}

	if storedLdapConfig.ServiceAccountPassword != "" {
		value, err := common.ReadFromSecret(p.secrets, storedLdapConfig.ServiceAccountPassword,
			strings.ToLower(client.LdapConfigFieldServiceAccountPassword))
		if err != nil {
			return nil, nil, err
		}
		storedLdapConfig.ServiceAccountPassword = value
	}

	return storedLdapConfig, p.caPool, nil
}

func (p *ldapProvider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []v3.Principal) (bool, error) {
	config, _, err := p.getLDAPConfig()
	if err != nil {
		logrus.Errorf("Error fetching ldap config: %v", err)
		return false, err
	}
	allowed, err := p.userMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipalID, groupPrincipals)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func (p *ldapProvider) getDNAndScopeFromPrincipalID(principalID string) (string, string, error) {
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return "", "", errors.Errorf("invalid id %v", principalID)
	}
	scope := parts[0]
	externalID := strings.TrimPrefix(parts[1], "//")

	return externalID, scope, nil
}

// if provider only enabled for search by a SAML provider
func (p *ldapProvider) samlSearchProvider() bool {
	if p.providerName == ShibbolethName {
		return true
	}
	return false
}

func (p *ldapProvider) samlSearchGetPrincipal(
	externalID string, scope string, config *v3.LdapConfig, caPool *x509.CertPool) (*v3.Principal, error) {

	if scope != p.userScope && scope != p.groupScope {
		return nil, fmt.Errorf("Invalid scope")
	}

	lConn, err := ldap.Connect(config, caPool)
	if err != nil {
		return nil, err
	}
	defer lConn.Close()

	err = ldap.AuthenticateServiceAccountUser(
		config.ServiceAccountPassword, config.ServiceAccountDistinguishedName, "", lConn)
	if err != nil {
		return nil, err
	}

	var searchRequest *ldapv2.SearchRequest
	var filter string
	if scope == p.userScope {
		filter = fmt.Sprintf("(&(%v=%v)(%v=%v))",
			ObjectClass, config.UserObjectClass, config.UserLoginAttribute, ldapv2.EscapeFilter(externalID))
		searchRequest = ldapv2.NewSearchRequest(config.UserSearchBase,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			filter, ldap.GetUserSearchAttributesForLDAP(ObjectClass, config), nil)
	} else {
		filter = fmt.Sprintf("(&(%v=%v)(%v=%v))",
			ObjectClass, config.GroupObjectClass, config.GroupDNAttribute, ldapv2.EscapeFilter(externalID))
		searchRequest = ldapv2.NewSearchRequest(config.GroupSearchBase,
			ldapv2.ScopeWholeSubtree, ldapv2.NeverDerefAliases, 0, 0, false,
			filter, ldap.GetGroupSearchAttributesForLDAP(ObjectClass, config), nil)
	}

	result, err := lConn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("saml search get principals search error: %s", err)
	}

	if len(result.Entries) < 1 {
		return nil, fmt.Errorf("No identities can be retrieved")
	} else if len(result.Entries) > 1 {
		return nil, fmt.Errorf("More than one result found")
	}

	entry := result.Entries[0]
	entryAttributes := entry.Attributes

	if scope == p.userScope {
		userLoginValues := ldap.GetAttributeValuesByName(entry.Attributes, config.UserLoginAttribute)
		if len(userLoginValues) > 0 {
			externalID = userLoginValues[0] // only support first
		}
	} else {
		groupDNValues := ldap.GetAttributeValuesByName(entry.Attributes, config.GroupDNAttribute)
		if len(groupDNValues) > 0 {
			externalID = groupDNValues[0] // only support first
		}
	}

	return ldap.AttributesToPrincipal(
		entryAttributes,
		externalID,
		scope,
		p.providerName,
		config.UserObjectClass,
		config.UserNameAttribute,
		config.UserLoginAttribute,
		config.GroupObjectClass,
		config.GroupNameAttribute)
}
