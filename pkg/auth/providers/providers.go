package providers

import (
	"context"
	"fmt"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"

	"sync"

	"github.com/rancher/rancher/pkg/auth/providers/activedirectory"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/types/client/management/v3"
	publicclient "github.com/rancher/types/client/management/v3public"
)

var (
	providers       = make(map[string]common.AuthProvider)
	providersByType = make(map[string]common.AuthProvider)
	confMu          sync.Mutex
)

func GetProvider(providerName string) (common.AuthProvider, error) {
	if provider, ok := providers[providerName]; ok {
		if provider != nil {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("No such provider '%s'", providerName)
}

func GetProviderByType(configType string) common.AuthProvider {
	return providersByType[configType]
}

func Configure(ctx context.Context, mgmt *config.ScaledContext) {
	confMu.Lock()
	defer confMu.Unlock()
	userMGR := common.NewUserManager(mgmt)
	var p common.AuthProvider
	p = local.Configure(ctx, mgmt)
	providers[local.Name] = p
	providersByType[client.LocalConfigType] = p
	providersByType[publicclient.LocalProviderType] = p

	p = github.Configure(ctx, mgmt, userMGR)
	providers[github.Name] = p
	providersByType[client.GithubConfigType] = p
	providersByType[publicclient.GithubProviderType] = p

	p = activedirectory.Configure(ctx, mgmt, userMGR)
	providers[activedirectory.Name] = p
	providersByType[client.ActiveDirectoryConfigType] = p
	providersByType[publicclient.ActiveDirectoryProviderType] = p
}

func AuthenticateUser(input interface{}, providerName string) (v3.Principal, []v3.Principal, map[string]string, error) {
	return providers[providerName].AuthenticateUser(input)
}

func GetPrincipal(principalID string, myToken v3.Token) (v3.Principal, error) {
	return providers[myToken.AuthProvider].GetPrincipal(principalID, myToken)
}

func SearchPrincipals(name, principalType string, myToken v3.Token) ([]v3.Principal, error) {
	return providers[myToken.AuthProvider].SearchPrincipals(name, principalType, myToken)
}
