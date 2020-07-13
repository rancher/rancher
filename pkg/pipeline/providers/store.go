package providers

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/store/empty"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/ref"
	client "github.com/rancher/rancher/pkg/types/client/project/v3"
	"github.com/rancher/rancher/pkg/types/config"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func setSourceCodeProviderStore(schema *types.Schema, apiContext *config.ScaledContext) {
	schema.Store = &sourceCodeProviderStore{
		scpConfigsRaw: apiContext.Project.SourceCodeProviderConfigs("").ObjectClient().UnstructuredClient(),
	}
}

type sourceCodeProviderStore struct {
	empty.Store
	scpConfigsRaw objectclient.GenericClient
}

func (s *sourceCodeProviderStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	ns, name := ref.Parse(id)
	o, err := s.scpConfigsRaw.GetNamespaced(ns, name, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	u, _ := o.(runtime.Unstructured)
	config := u.UnstructuredContent()
	if t := convert.ToString(config[client.SourceCodeProviderFieldType]); t != "" && providersByType[t] != nil {
		return providersByType[t].TransformToSourceCodeProvider(config), nil
	}

	return nil, httperror.NewAPIError(httperror.NotFound, "")
}

func (s *sourceCodeProviderStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	rrr, _ := s.scpConfigsRaw.List(v1.ListOptions{})
	var result []map[string]interface{}
	list, _ := rrr.(*unstructured.UnstructuredList)
	for _, i := range list.Items {
		if t := convert.ToString(i.Object[client.SourceCodeProviderFieldType]); t != "" && providersByType[t] != nil {
			if enabled, ok := i.Object[client.SourceCodeProviderConfigFieldEnabled].(bool); ok && enabled {
				result = append(result, providersByType[t].TransformToSourceCodeProvider(i.Object))
			}
		}
	}
	return result, nil
}

func (s *sourceCodeProviderStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	result, err := s.Update(apiContext, schema, data, id)
	if err != nil {
		return nil, err
	}
	return result, nil
}
