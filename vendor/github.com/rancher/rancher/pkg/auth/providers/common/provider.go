package common

import (
	"context"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

type AuthProvider interface {
	GetName() string
	AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error)
	SearchPrincipals(name, principalType string, myToken v3.Token) ([]v3.Principal, error)
	GetPrincipal(principalID string, token v3.Token) (v3.Principal, error)
	CustomizeSchema(schema *types.Schema)
	TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error)
	RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error)
	CanAccessWithGroupProviders(userPrincipalID string, groups []v3.Principal) (bool, error)
}
