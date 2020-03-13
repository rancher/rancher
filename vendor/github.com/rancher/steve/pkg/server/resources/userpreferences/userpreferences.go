package userpreferences

import (
	"net/http"

	"github.com/rancher/steve/pkg/schemaserver/store/empty"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/steve/pkg/server/store/proxy"
	"github.com/rancher/wrangler/pkg/name"
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
		schema.ResourceMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{http.MethodGet, http.MethodPut, http.MethodDelete}
		schema.Store = New(cg)
	})
}

func New(cg proxy.ClientGetter) types.Store {
	return &Store{
		rancher: &rancherPrefStore{
			cg: cg,
		},
		configMapStore: &configMapStore{
			cg: cg,
		},
	}
}

type Store struct {
	empty.Store
	rancher        *rancherPrefStore
	configMapStore *configMapStore
}

func isRancher(apiOp *types.APIRequest) bool {
	return apiOp.Schemas.LookupSchema(rancherSchema) != nil
}

func (e *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	if isRancher(apiOp) {
		return e.rancher.ByID(apiOp, schema, id)
	}
	return e.configMapStore.ByID(apiOp, schema, id)
}

func (e *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	if isRancher(apiOp) {
		return e.rancher.List(apiOp, schema)
	}
	return e.configMapStore.List(apiOp, schema)
}

func (e *Store) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	if isRancher(apiOp) {
		return e.rancher.Update(apiOp, schema, data, id)
	}
	return e.configMapStore.Update(apiOp, schema, data, id)
}

func (e *Store) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	if isRancher(apiOp) {
		return e.rancher.Delete(apiOp, schema, id)
	}
	return e.configMapStore.Delete(apiOp, schema, id)
}

func prefName(u user.Info) string {
	return name.SafeConcatName("pref", u.GetName())
}

func getUser(apiOp *types.APIRequest) user.Info {
	u, ok := request.UserFrom(apiOp.Context())
	if !ok {
		u = &user.DefaultInfo{
			Name: "dashboard-user",
		}
	}
	return u
}
