package publicapi

import (
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/store/empty"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/settings"
	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/rancher/rancher/pkg/types/config"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func setAuthProvidersStore(schema *types.Schema, apiContext *config.ScaledContext) {
	schema.Store = &authProvidersStore{
		authConfigsRaw: apiContext.Management.AuthConfigs("").ObjectClient().UnstructuredClient(),
	}
}

type authProvidersStore struct {
	empty.Store
	authConfigsRaw objectclient.GenericClient
}

func (s *authProvidersStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	o, err := s.authConfigsRaw.Get(id, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	u, _ := o.(runtime.Unstructured)
	config := u.UnstructuredContent()
	if t, ok := config["type"].(string); ok && t != "" {
		config[".host"] = util.GetHost(apiContext.Request)
		provider, err := providers.GetProviderByType(t).TransformToAuthProvider(config)
		if err != nil {
			return nil, err
		}
		return provider, nil
	}

	return nil, httperror.NewAPIError(httperror.NotFound, "")
}

func (s *authProvidersStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	rrr, _ := s.authConfigsRaw.List(v1.ListOptions{})
	var result []map[string]interface{}
	list, _ := rrr.(*unstructured.UnstructuredList)
	for _, i := range list.Items {
		if t, ok := i.Object["type"].(string); ok && t != "" {
			if enabled, ok := i.Object["enabled"].(bool); ok && enabled {
				i.Object[".host"] = util.GetHost(apiContext.Request)
				provider, err := providers.GetProviderByType(t).TransformToAuthProvider(i.Object)
				if err != nil {
					return result, err
				}
				result = append(result, provider)
			}
		}
	}
	return result, nil
}

func (s *authProvidersStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	result, err := s.Update(apiContext, schema, data, id)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(settings.FirstLogin.Get(), "true") {
		if err := settings.FirstLogin.Set("false"); err != nil {
			return nil, err
		}
	}
	return result, nil
}
