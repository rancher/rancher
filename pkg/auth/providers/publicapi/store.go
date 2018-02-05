package publicapi

import (
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setAuthProvidersStore(schema *types.Schema, mgmt *config.ManagementContext) {
	schema.Store = &authProvidersStore{
		mgmt: mgmt,
	}
}

type authProvidersStore struct {
	mgmt *config.ManagementContext
}

func (s *authProvidersStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	config, err := s.mgmt.Management.AuthConfigs("").Get(id, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"id": config.Name, "type": strings.Replace(config.Type, "Config", "Provider", -1)}, nil
}

func (s *authProvidersStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	configs, err := s.mgmt.Management.AuthConfigs("").List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	for _, ac := range configs.Items {
		result = append(result, map[string]interface{}{"id": ac.Name, "type": strings.Replace(ac.Type, "Config", "Provider", -1)})
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
