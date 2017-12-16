package providers

import (
	"context"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"

	"github.com/rancher/auth/providers/local"
)

//Providers map
var providers map[string]PrincipalProvider
var providerOrderList []string

func init() {
	providerOrderList = []string{"local"}
	providers = make(map[string]PrincipalProvider)
}

//PrincipalProvider interfacse defines what methods an identity provider should implement
type PrincipalProvider interface {
	GetName() string
	AuthenticateUser(jsonInput v3.LoginInput) (v3.Principal, []v3.Principal, int, error)
}

func Configure(ctx context.Context, mgmtCtx *config.ManagementContext) {
	for _, name := range providerOrderList {
		switch name {
		case "local":
			providers[name] = local.Configure(ctx, mgmtCtx)
		}
	}
}

func AuthenticateUser(jsonInput v3.LoginInput) (v3.Principal, []v3.Principal, int, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal
	var status int
	var err error

	for _, name := range providerOrderList {
		switch name {
		case "local":
			userPrincipal, groupPrincipals, status, err = providers[name].AuthenticateUser(jsonInput)
		}
	}
	return userPrincipal, groupPrincipals, status, err
}
