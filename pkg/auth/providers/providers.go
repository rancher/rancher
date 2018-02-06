package providers

import (
	"context"
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"

	"sync"

	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/local"
)

//Providers map
var (
	providers         map[string]PrincipalProvider
	providerOrderList []string
	confMu            sync.Mutex
)

func init() {
	providerOrderList = []string{"github", "local"}
	providers = make(map[string]PrincipalProvider)
}

//PrincipalProvider interfacse defines what methods an identity provider should implement
type PrincipalProvider interface {
	GetName() string
	AuthenticateUser(input interface{}) (v3.Principal, []v3.Principal, map[string]string, int, error)
	SearchPrincipals(name string, myToken v3.Token) ([]v3.Principal, int, error)
	ConfigActionHandler(actionName string, action *types.Action, request *types.APIContext) error
}

func GetProvider(providerName string) (PrincipalProvider, error) {
	if provider, ok := providers[providerName]; ok {
		if provider != nil {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("No such provider '%s'", providerName)
}

func Configure(ctx context.Context, mgmt *config.ManagementContext) {
	confMu.Lock()
	defer confMu.Unlock()
	userMGR := common.NewUserManager(mgmt)
	for _, providerName := range providerOrderList {
		if _, exists := providers[providerName]; !exists {
			switch providerName {
			case "local":
				providers[providerName] = local.Configure(ctx, mgmt)
			case "github":
				providers[providerName] = github.Configure(ctx, mgmt, userMGR)
			}
		}
	}
}

func AuthenticateUser(input interface{}, providerName string) (v3.Principal, []v3.Principal, map[string]string, int, error) {
	return providers[providerName].AuthenticateUser(input)
}

func SearchPrincipals(name string, myToken v3.Token) ([]v3.Principal, int, error) {
	return providers[myToken.AuthProvider].SearchPrincipals(name, myToken)
}
