package providers

import (
	"context"
	"fmt"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"

	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/rancher/pkg/auth/providers/local"
)

//Providers map
var providers map[string]PrincipalProvider
var providerOrderList []string

func init() {
	providerOrderList = []string{"github", "local"}
	providers = make(map[string]PrincipalProvider)
}

//PrincipalProvider interfacse defines what methods an identity provider should implement
type PrincipalProvider interface {
	GetName() string
	AuthenticateUser(jsonInput v3.LoginInput) (v3.Principal, []v3.Principal, map[string]string, int, error)
	SearchPrincipals(name string, myToken v3.Token) ([]v3.Principal, int, error)
}

func GetProvider(providerName string) (PrincipalProvider, error) {
	if provider, ok := providers[providerName]; ok {
		if provider != nil {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("No such provider '%s'", providerName)
}

func Configure(ctx context.Context, mgmtCtx *config.ManagementContext) {
	for _, providerName := range providerOrderList {
		if _, exists := providers[providerName]; !exists {
			switch providerName {
			case "local":
				providers[providerName] = local.Configure(ctx, mgmtCtx)
			case "github":
				providers[providerName] = github.Configure(ctx, mgmtCtx)
			}
		}
	}
}

func AuthenticateUser(jsonInput v3.LoginInput) (v3.Principal, []v3.Principal, map[string]string, int, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal
	var providerInfo = make(map[string]string)
	var status int
	var err error
	var providerName string

	if jsonInput.GithubCredential.Code != "" {
		providerName = "github"
	} else if jsonInput.LocalCredential.Username != "" {
		providerName = "local"
	}

	userPrincipal, groupPrincipals, providerInfo, status, err = providers[providerName].AuthenticateUser(jsonInput)

	return userPrincipal, groupPrincipals, providerInfo, status, err
}

func SearchPrincipals(name string, myToken v3.Token) ([]v3.Principal, int, error) {
	principals := make([]v3.Principal, 0)
	var status int
	var err error

	principals, status, err = providers[myToken.AuthProvider].SearchPrincipals(name, myToken)

	return principals, status, err
}
