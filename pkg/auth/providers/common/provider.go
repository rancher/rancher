package common

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type AuthProvider interface {
	GetName() string
	AuthenticateUser(input interface{}) (v3.Principal, []v3.Principal, map[string]string, int, error)
	SearchPrincipals(name, principalType string, myToken v3.Token) ([]v3.Principal, int, error)
	CustomizeSchema(schema *types.Schema)
	TransformToAuthProvider(authConfig map[string]interface{}) map[string]interface{}
}
