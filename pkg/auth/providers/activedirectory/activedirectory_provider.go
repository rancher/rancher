package activedirectory

import (
	"context"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
)

const (
	Name                               = "activedirectory"
	UserScope                          = Name + "_user"
	GroupScope                         = Name + "_group"
	ObjectClass                        = "objectClass"
	MemberOfAttribute                  = "memberOf"
	StatusConfigMapName                = "ad-guid-migration"
	StatusConfigMapNamespace           = "cattle-system"
	StatusMigrationField               = "ad-guid-migration-status"
	StatusMigrationFinished            = "Finished"
	StatusMigrationRunning             = "Running"
	StatusMigrationFinishedWithSkipped = "FinishedWithSkipped"
	StatusMigrationFinishedWithMissing = "FinishedWithMissing"
	StatusMigrationFailed              = "Failed"
	StatusLoginDisabled                = "login is disabled while migration is running"
	StatusACMigrationRunning           = "migration-ad-guid-migration-status"
)

var scopes = []string{UserScope, GroupScope}

type adProvider struct {
	ctx         context.Context
	authConfigs v3.AuthConfigInterface
	configMaps  corev1.ConfigMapLister
	secrets     corev1.SecretInterface
	userMGR     user.Manager
	certs       string
	caPool      *x509.CertPool
	tokenMGR    *tokens.Manager
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	return &adProvider{
		ctx:         ctx,
		authConfigs: mgmtCtx.Management.AuthConfigs(""),
		configMaps:  mgmtCtx.Core.ConfigMaps("").Controller().Lister(),
		secrets:     mgmtCtx.Core.Secrets(""),
		userMGR:     userMGR,
		tokenMGR:    tokenMGR,
	}
}

func (p *adProvider) GetName() string {
	return Name
}

func (p *adProvider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = p.actionHandler
	schema.Formatter = p.formatter
}

func (p *adProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	ap := common.TransformToAuthProvider(authConfig)
	defaultDomain := ""
	if dld, ok := authConfig[client.ActiveDirectoryProviderFieldDefaultLoginDomain].(string); ok {
		defaultDomain = dld
	}
	ap[client.ActiveDirectoryProviderFieldDefaultLoginDomain] = defaultDomain
	return ap, nil
}

func (p *adProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	login, ok := input.(*v32.BasicLogin)
	if !ok {
		return v3.Principal{}, nil, "", errors.New("unexpected input type")
	}

	config, caPool, err := p.getActiveDirectoryConfig()
	if err != nil {
		return v3.Principal{}, nil, "", errors.New("can't find authprovider")
	}

	// If a migration is running, we need to block logins and indicate why we are doing so
	if config.Annotations != nil && config.Annotations[StatusACMigrationRunning] == StatusMigrationRunning {
		return v3.Principal{}, nil, "", httperror.WrapAPIError(err, httperror.ClusterUnavailable, StatusLoginDisabled)
	}

	principal, groupPrincipal, err := p.loginUser(login, config, caPool, false)
	if err != nil {
		return v3.Principal{}, nil, "", err
	}

	return principal, groupPrincipal, "", err
}

func (p *adProvider) SearchPrincipals(searchKey, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal
	var err error

	config, caPool, err := p.getActiveDirectoryConfig()
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
				principal.MemberOf = p.tokenMGR.IsMemberOf(myToken, principal)
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

	externalID, scope, err := p.getDNAndScopeFromPrincipalID(principalID)
	if err != nil {
		return v3.Principal{}, err
	}

	principal, err := p.getPrincipal(externalID, scope, config, caPool)
	if err != nil {
		return v3.Principal{}, err
	}
	if p.isThisUserMe(token.UserPrincipal, *principal) {
		principal.Me = true
	}
	return *principal, err
}

func (p *adProvider) isThisUserMe(me v3.Principal, other v3.Principal) bool {
	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}

func (p *adProvider) getActiveDirectoryConfig() (*v32.ActiveDirectoryConfig, *x509.CertPool, error) {
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

	storedADConfig := &v32.ActiveDirectoryConfig{}
	err = common.Decode(storedADConfigMap, storedADConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to decode Active Directory Config: %w", err)
	}

	if p.certs != storedADConfig.Certificate || p.caPool == nil {
		pool, err := newCAPool(storedADConfig.Certificate)
		if err != nil {
			return nil, nil, err
		}
		p.certs = storedADConfig.Certificate
		p.caPool = pool
	}

	if storedADConfig.ServiceAccountPassword != "" {
		value, err := common.ReadFromSecret(p.secrets, storedADConfig.ServiceAccountPassword,
			strings.ToLower(v3client.ActiveDirectoryConfigFieldServiceAccountPassword))
		if err != nil {
			return nil, nil, err
		}
		storedADConfig.ServiceAccountPassword = value
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

func (p *adProvider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []v3.Principal) (bool, error) {
	config, _, err := p.getActiveDirectoryConfig()
	if err != nil {
		logrus.Errorf("Error fetching AD config: %v", err)
		return false, err
	}
	allowed, err := p.userMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipalID, groupPrincipals)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func (p *adProvider) getDNAndScopeFromPrincipalID(principalID string) (string, string, error) {
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return "", "", errors.Errorf("invalid id %v", principalID)
	}
	scope := parts[0]
	externalID := strings.TrimPrefix(parts[1], "//")
	return externalID, scope, nil
}

func (p *adProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	extras := make(map[string][]string)
	if userPrincipal.Name != "" {
		extras[common.UserAttributePrincipalID] = []string{userPrincipal.Name}
	}
	if userPrincipal.LoginName != "" {
		extras[common.UserAttributeUserName] = []string{userPrincipal.LoginName}
	}
	return extras
}

type LoginDisabledError struct{}

// Error provides a string representation of an LdapErrorNotFound
func (e LoginDisabledError) Error() string {
	return StatusLoginDisabled
}

// IsDisabledProvider checks if the Azure Active Directory provider is currently disabled in Rancher.
func (p *adProvider) IsDisabledProvider() (bool, error) {
	adConfig, _, err := p.getActiveDirectoryConfig()
	if err != nil {
		return false, err
	}
	return !adConfig.Enabled, nil
}
