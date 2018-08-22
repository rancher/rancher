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
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	OpenLdapName       = "openldap"
	FreeIpaName        = "freeipa"
	OpenLdapUserScope  = OpenLdapName + "_user"
	OpenLdapGroupScope = OpenLdapName + "_group"
	FreeIpaUserScope   = FreeIpaName + "_user"
	FreeIpaGroupScope  = FreeIpaName + "_group"
)

var openLdapScopes = []string{OpenLdapUserScope, OpenLdapGroupScope}
var freeIpaScopes = []string{FreeIpaUserScope, FreeIpaGroupScope}

type ldapProvider struct {
	ctx                   context.Context
	authConfigs           v3.AuthConfigInterface
	userMGR               user.Manager
	tokenMGR              *tokens.Manager
	certs                 string
	caPool                *x509.CertPool
	providerName          string
	testAndApplyInputType string
	userScope             string
	groupScope            string
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager, providerName string, testAndApplyInputType string, userScope string, groupScope string) common.AuthProvider {
	return &ldapProvider{
		ctx:                   ctx,
		authConfigs:           mgmtCtx.Management.AuthConfigs(""),
		userMGR:               userMGR,
		tokenMGR:              tokenMGR,
		providerName:          providerName,
		testAndApplyInputType: testAndApplyInputType,
		userScope:             userScope,
		groupScope:            groupScope}
}

func (p *ldapProvider) GetName() string {
	return p.providerName
}

func (p *ldapProvider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = p.actionHandler
	schema.Formatter = p.formatter
}

func (p *ldapProvider) TransformToAuthProvider(authConfig map[string]interface{}) map[string]interface{} {
	ldap := common.TransformToAuthProvider(authConfig)
	return ldap
}

func (p *ldapProvider) AuthenticateUser(input interface{}) (v3.Principal, []v3.Principal, string, error) {
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

func (p *ldapProvider) SearchPrincipals(searchKey, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal
	var err error

	config, caPool, err := p.getLDAPConfig()
	if err != nil {
		return principals, nil
	}

	lConn, err := p.ldapConnection(config, caPool)
	if err != nil {
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
		return v3.Principal{}, nil
	}

	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return v3.Principal{}, errors.Errorf("invalid id %v", principalID)
	}
	scope := parts[0]
	externalID := strings.TrimPrefix(parts[1], "//")

	principal, err := p.getPrincipal(externalID, scope, config, caPool)
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

	mapstructure.Decode(storedLdapConfigMap, storedLdapConfig)

	metadataMap, ok := storedLdapConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("failed to retrieve %s metadata, cannot read k8s Unstructured data", p.providerName)
	}

	objectMeta := &metav1.ObjectMeta{}
	mapstructure.Decode(metadataMap, objectMeta)
	storedLdapConfig.ObjectMeta = *objectMeta

	if p.certs != storedLdapConfig.Certificate || p.caPool == nil {
		pool, err := newCAPool(storedLdapConfig.Certificate)
		if err != nil {
			return nil, nil, err
		}
		p.certs = storedLdapConfig.Certificate
		p.caPool = pool
	}

	return storedLdapConfig, p.caPool, nil
}

func newCAPool(cert string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	pool.AppendCertsFromPEM([]byte(cert))
	return pool, nil
}
