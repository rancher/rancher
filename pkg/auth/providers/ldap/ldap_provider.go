package ldap

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/common/ldap"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	OpenLdapName   = "openldap"
	FreeIpaName    = "freeipa"
	ShibbolethName = "shibboleth"
	ObjectClass    = "objectClass"
	OKTAName       = "okta"
)

// An ErrorNotConfigured indicates that the requested LDAP operation
// failed due to missing or incomplete configuration.
type ErrorNotConfigured struct{}

// Error provides a string representation of an ErrorNotConfigured
func (e ErrorNotConfigured) Error() string {
	return "not configured"
}

var (
	testAndApplyInputTypes = map[string]string{
		FreeIpaName:  client.FreeIpaTestAndApplyInputType,
		OpenLdapName: client.OpenLdapTestAndApplyInputType,
	}

	// empty string for inline
	ldapConfigKey = map[string]string{
		FreeIpaName:    "",
		OpenLdapName:   "",
		ShibbolethName: client.ShibbolethConfigFieldOpenLdapConfig,
		OKTAName:       client.OKTAConfigFieldOpenLdapConfig,
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

	return ldapProvider.getLDAPConfig(ldapProvider.authConfigs.ObjectClient().UnstructuredClient())
}

// IsNotConfigured checks whether this error indicates a missing LDAP configuration.
func IsNotConfigured(err error) bool {
	return errors.Is(err, ErrorNotConfigured{})
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

func toBasicLogin(input interface{}) (*v32.BasicLogin, error) {
	login, ok := input.(*v32.BasicLogin)
	if !ok {
		return nil, errors.New("unexpected input type")
	}
	return login, nil
}

// AuthenticateUser takes in a context and user credentials, and authenticates the user against an LDAP server.
// Returns principal, slice of group principals, and any errors encountered.
func (p *ldapProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	login, err := toBasicLogin(input)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	config, caPool, err := p.getLDAPConfig(p.authConfigs.ObjectClient().UnstructuredClient())
	if err != nil {
		return v3.Principal{}, nil, "", errors.New("can't find authprovider")
	}

	lConn, err := ldap.Connect(config, caPool)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}
	defer lConn.Close()

	principal, groupPrincipal, err := p.loginUser(lConn, login, config, caPool)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	return principal, groupPrincipal, "", err
}

// searchKey can be user PrincipalID e.g. shibboleth_user://username with principalType of group for group search by user
func (p *ldapProvider) SearchPrincipals(searchKey, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal
	var err error

	config, caPool, err := p.getLDAPConfig(p.authConfigs.ObjectClient().UnstructuredClient())
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
	config, caPool, err := p.getLDAPConfig(p.authConfigs.ObjectClient().UnstructuredClient())
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

func (p *ldapProvider) getLDAPConfig(genericClient objectclient.GenericClient) (*v3.LdapConfig, *x509.CertPool, error) {
	// TODO See if this can be simplified. also, this makes an api call everytime. find a better way
	authConfigObj, err := genericClient.Get(p.providerName, metav1.GetOptions{})
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
			return nil, nil, ErrorNotConfigured{}
		}

		storedLdapConfigMap = subLdapConfig.(map[string]interface{})
		err = common.Decode(storedLdapConfigMap, storedLdapConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to decode Ldap Config: %w", err)
		}
		if len(storedLdapConfig.Servers) < 1 {
			return storedLdapConfig, nil, ErrorNotConfigured{}
		}
	} else {
		err = common.Decode(storedLdapConfigMap, storedLdapConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to decode Ldap Config: %w", err)
		}
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
	config, _, err := p.getLDAPConfig(p.authConfigs.ObjectClient().UnstructuredClient())
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
		return "", "", fmt.Errorf("invalid id %v", principalID)
	}
	scope := parts[0]
	externalID := strings.TrimPrefix(parts[1], "//")

	return externalID, scope, nil
}

// if provider only enabled for search by a SAML provider
func (p *ldapProvider) samlSearchProvider() bool {
	return ShibbolethName == p.providerName || OKTAName == p.providerName
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

	var searchRequest *ldapv3.SearchRequest
	var filter string
	if scope == p.userScope {
		filter = fmt.Sprintf("(&(%v=%v)(%v=%v))",
			ObjectClass, config.UserObjectClass, config.UserLoginAttribute, ldapv3.EscapeFilter(externalID))
		searchRequest = ldapv3.NewSearchRequest(config.UserSearchBase,
			ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 0, 0, false,
			filter, ldap.GetUserSearchAttributesForLDAP(ObjectClass, config), nil)
	} else {
		filter = fmt.Sprintf("(&(%v=%v)(%v=%v))",
			ObjectClass, config.GroupObjectClass, config.GroupDNAttribute, ldapv3.EscapeFilter(externalID))
		searchRequest = ldapv3.NewSearchRequest(config.GroupSearchBase,
			ldapv3.ScopeWholeSubtree, ldapv3.NeverDerefAliases, 0, 0, false,
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

func (p *ldapProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	extras := make(map[string][]string)
	if userPrincipal.Name != "" {
		extras[common.UserAttributePrincipalID] = []string{userPrincipal.Name}
	}
	if userPrincipal.LoginName != "" {
		extras[common.UserAttributeUserName] = []string{userPrincipal.LoginName}
	}
	return extras
}

// IsDisabledProvider checks if the LDAP auth provider is currently disabled in Rancher.
func (p *ldapProvider) IsDisabledProvider() (bool, error) {
	ldapConfig, _, err := p.getLDAPConfig(p.authConfigs.ObjectClient().UnstructuredClient())
	if err != nil {
		return false, err
	}
	return !ldapConfig.Enabled, nil
}
