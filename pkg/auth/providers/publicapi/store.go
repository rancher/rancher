package publicapi

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func setAuthProvidersStore(schema *types.Schema, mgmt *config.ManagementContext) {
	schema.Store = &authProvidersStore{
		mgmt: mgmt,
	}
}

type authProvidersStore struct {
	mgmt *config.ManagementContext
}

func (s *authProvidersStore) Context() types.StorageContext {
	return types.DefaultStorageContext
}

func (s *authProvidersStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	o, err := s.mgmt.Management.AuthConfigs("").ObjectClient().UnstructuredClient().Get(id, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	u, _ := o.(runtime.Unstructured)
	config := u.UnstructuredContent()
	if t, ok := config["type"].(string); ok && t != "" {
		return providers.GetProviderByType(t).TransformToAuthProvider(config), nil
	}

	return nil, httperror.NewAPIError(httperror.NotFound, "")
}

func (s *authProvidersStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	rrr, _ := s.mgmt.Management.AuthConfigs("").ObjectClient().UnstructuredClient().List(v1.ListOptions{})
	var result []map[string]interface{}
	list, _ := rrr.(*unstructured.UnstructuredList)
	for _, i := range list.Items {
		if t, ok := i.Object["type"].(string); ok && t != "" {
			if enabled, ok := i.Object["enabled"].(bool); ok && enabled {
				result = append(result, providers.GetProviderByType(t).TransformToAuthProvider(i.Object))
			}
		}
	}
	return result, nil
}

func (s *authProvidersStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	return nil, httperror.NewAPIError(httperror.MethodNotAllowed, "Method not allowed")
}

func (s *authProvidersStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	return nil, httperror.NewAPIError(httperror.MethodNotAllowed, "Method not allowed")
}

func (s *authProvidersStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	return nil, httperror.NewAPIError(httperror.MethodNotAllowed, "Method not allowed")
}

func (s *authProvidersStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	return nil, httperror.NewAPIError(httperror.MethodNotAllowed, "Method not allowed")
}
