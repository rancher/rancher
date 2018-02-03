package activedirectory

import (
	"context"
	"crypto/x509"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/rancher/pkg/auth/providers/ldap"
	"k8s.io/apimachinery/pkg/runtime"
)

//Constants for ad
const (
	Name                 = "activedirectory"
	UserScope            = Name + "_user"
	GroupScope           = Name + "_group"
	MemberOfAttribute    = "memberOf"
	ObjectClassAttribute = "objectClass"
)

var scopes = []string{UserScope, GroupScope}

var adConstantsConfig = &ldap.ConstantsConfig{
	UserScope:            UserScope,
	GroupScope:           GroupScope,
	Scopes:               scopes,
	MemberOfAttribute:    MemberOfAttribute,
	ObjectClassAttribute: ObjectClassAttribute,
}

//AProvider implements an PrincipalProvider for AD
type AProvider struct {
	ctx         context.Context
	authConfigs v3.AuthConfigInterface
	adClient    *LClient
}

func Configure(ctx context.Context, mgmtCtx *config.ManagementContext) *AProvider {
	pool, err := x509.SystemCertPool()
	if err != nil {
		logrus.Errorf("Error in loading ldap certs: %v", err)
		return nil
	}
	adConstantsConfig.CAPool = pool

	adClient := &LClient{ConstantsConfig: adConstantsConfig}
	return &AProvider{
		ctx:         ctx,
		authConfigs: mgmtCtx.Management.AuthConfigs(""),
		adClient:    adClient,
	}
}

//GetName returns the name of the provider
func (a *AProvider) GetName() string {
	return Name
}

func (a *AProvider) getActiveDirectoryConfigCR() (*v3.ActiveDirectoryConfig, error) {

	authConfigObj, err := a.authConfigs.ObjectClient().UnstructuredClient().Get("activedirectory", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve ActiveDirectoryConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve ActiveDirectoryConfig, cannot read k8s Unstructured data")
	}
	storedADConfigMap := u.UnstructuredContent()

	storedADConfig := &v3.ActiveDirectoryConfig{}
	mapstructure.Decode(storedADConfigMap, storedADConfig)

	metadataMap, ok := storedADConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to retrieve ActiveDirectoryConfig metadata, cannot read k8s Unstructured data")
	}

	typemeta := &metav1.ObjectMeta{}
	mapstructure.Decode(metadataMap, typemeta)
	storedADConfig.ObjectMeta = *typemeta

	return storedADConfig, nil
}

func (a *AProvider) SaveActiveDirectoryConfig(config *v3.ActiveDirectoryConfig) error {
	storedConfig, err := a.getActiveDirectoryConfigCR()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = client.ActiveDirectoryConfigType
	config.ObjectMeta = storedConfig.ObjectMeta

	logrus.Debugf("updating githubConfig")
	_, err = a.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}

func (a *AProvider) AuthenticateUser(loginInput v3.LoginInput) (v3.Principal, []v3.Principal, map[string]string, int, error) {
	return a.LoginUser(loginInput.ActiveDirectoryCredential, nil)
}

func (a *AProvider) LoginUser(adCredential v3.ActiveDirectoryCredential, config *v3.ActiveDirectoryConfig) (v3.Principal, []v3.Principal, map[string]string, int, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal
	var providerInfo = make(map[string]string)
	var err error

	if config == nil {
		config, err = a.getActiveDirectoryConfigCR()
		if err != nil {
			return userPrincipal, groupPrincipals, providerInfo, 401, err
		}
	}

	return a.adClient.LoginUser(adCredential, config)
}

func (a *AProvider) SearchPrincipals(searchKey string, myToken v3.Token) ([]v3.Principal, int, error) {
	var principals []v3.Principal
	var err error

	//is this ad token?
	if myToken.AuthProvider != a.GetName() {
		return principals, 0, nil
	}

	config, err := a.getActiveDirectoryConfigCR()
	if err != nil {
		return principals, 0, nil
	}

	principals, err = a.adClient.SearchPrincipals(searchKey, true, config)
	if err == nil {
		for _, p := range principals {
			if p.Kind == "user" {
				if a.isThisUserMe(myToken.UserPrincipal, p) {
					p.Me = true
				}
			} else if p.Kind == "group" {
				if a.isMemberOf(myToken.GroupPrincipals, p) {
					p.MemberOf = true
				}
			}
		}
	}

	return principals, 0, nil
}

func (a *AProvider) isThisUserMe(me v3.Principal, other v3.Principal) bool {

	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.Kind == other.Kind {
		return true
	}
	return false
}

func (a *AProvider) isMemberOf(myGroups []v3.Principal, other v3.Principal) bool {

	for _, mygroup := range myGroups {
		if mygroup.ObjectMeta.Name == other.ObjectMeta.Name && mygroup.Kind == other.Kind {
			return true
		}
	}
	return false
}
