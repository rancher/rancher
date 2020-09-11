package userpreferences

import (
	"net/http"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/stores/proxy"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type UserPreference struct {
	Data map[string]string `json:"data"`
}

func Register(schemas *types.APISchemas, cg proxy.ClientGetter) {
	schemas.InternalSchemas.TypeName("userpreference", UserPreference{})
	schemas.MustImportAndCustomize(UserPreference{}, func(schema *types.APISchema) {
		schema.CollectionMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{http.MethodGet, http.MethodPut, http.MethodDelete}
		schema.Store = &rancherPrefStore{
			cg: cg,
		}
	})
}

func getUser(apiOp *types.APIRequest) (user.Info, bool) {
	return request.UserFrom(apiOp.Context())
}
