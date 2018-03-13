package activedirectory

import (
	"context"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/client/management/v3public"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	Name                 = "activedirectory"
	UserScope            = Name + "_user"
	GroupScope           = Name + "_group"
	MemberOfAttribute    = "memberOf"
	ObjectClassAttribute = "objectClass"
)

var scopes = []string{UserScope, GroupScope}

type adProvider struct {
	ctx         context.Context
	authConfigs v3.AuthConfigInterface
	userMGR     user.Manager
	certs       string
	caPool      *x509.CertPool
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager) common.AuthProvider {
	return &adProvider{
		ctx:         ctx,
		authConfigs: mgmtCtx.Management.AuthConfigs(""),
		userMGR:     userMGR,
	}
}

func (p *adProvider) GetName() string {
	return Name
}

func (p *adProvider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = p.actionHandler
	schema.Formatter = p.formatter
}

func (p *adProvider) TransformToAuthProvider(authConfig map[string]interface{}) map[string]interface{} {
	ap := common.TransformToAuthProvider(authConfig)
	defaultDomain := ""
	if dld, ok := authConfig[client.ActiveDirectoryProviderFieldDefaultLoginDomain].(string); ok {
		defaultDomain = dld
	}
	ap[client.ActiveDirectoryProviderFieldDefaultLoginDomain] = defaultDomain
	return ap
}

func (p *adProvider) AuthenticateUser(input interface{}) (v3.Principal, []v3.Principal, map[string]string, error) {
	login, ok := input.(*v3public.BasicLogin)
	if !ok {
		return v3.Principal{}, nil, nil, errors.New("unexpected input type")
	}

	config, caPool, err := p.getActiveDirectoryConfig()
	if err != nil {
		return v3.Principal{}, nil, nil, errors.New("can't find authprovider")
	}

	return p.loginUser(login, config, caPool)
}

func (p *adProvider) SearchPrincipals(searchKey, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal
	var err error

	// TODO use principalType in search

	config, caPool, err := p.getActiveDirectoryConfig()
	if err != nil {
		return principals, nil
	}

	principals, err = p.searchPrincipals(searchKey, principalType, config, caPool)
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

func (p *adProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	config, caPool, err := p.getActiveDirectoryConfig()
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
	return *principal, err
}

func (p *adProvider) isThisUserMe(me v3.Principal, other v3.Principal) bool {
	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}

func (p *adProvider) isMemberOf(myGroups []v3.Principal, other v3.Principal) bool {

	for _, mygroup := range myGroups {
		if mygroup.ObjectMeta.Name == other.ObjectMeta.Name && mygroup.PrincipalType == other.PrincipalType {
			return true
		}
	}
	return false
}

func (p *adProvider) getActiveDirectoryConfig() (*v3.ActiveDirectoryConfig, *x509.CertPool, error) {
	// TODO See if this can be simplified. also, this makes an api call everytime. find a better way
	authConfigObj, err := p.authConfigs.ObjectClient().UnstructuredClient().Get("activedirectory", metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve ActiveDirectoryConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, nil, fmt.Errorf("failed to retrieve ActiveDirectoryConfig, cannot read k8s Unstructured data")
	}
	storedADConfigMap := u.UnstructuredContent()

	storedADConfig := &v3.ActiveDirectoryConfig{}
	mapstructure.Decode(storedADConfigMap, storedADConfig)

	metadataMap, ok := storedADConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("failed to retrieve ActiveDirectoryConfig metadata, cannot read k8s Unstructured data")
	}

	typemeta := &metav1.ObjectMeta{}
	mapstructure.Decode(metadataMap, typemeta)
	storedADConfig.ObjectMeta = *typemeta

	if p.certs != storedADConfig.Certificate || p.caPool == nil {
		pool, err := newCAPool(storedADConfig.Certificate)
		if err != nil {
			return nil, nil, err
		}
		p.certs = storedADConfig.Certificate
		p.caPool = pool
	}

	return storedADConfig, p.caPool, nil
}

func newCAPool(cert string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	pool.AppendCertsFromPEM([]byte(cert))
	return pool, nil
}
