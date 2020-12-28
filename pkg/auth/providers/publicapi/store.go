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
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
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

func setAuthTokensStore(schema *types.Schema, apiContext *config.ScaledContext) {
	schema.Store = &authTokensStore{
		tokens: apiContext.Management.SamlTokens(""),
	}
}

type authTokensStore struct {
	empty.Store
	tokens v3.SamlTokenInterface
}

func (t *authTokensStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	token, err := t.tokens.GetNamespaced(namespace.GlobalNamespace, id, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	generated := transformToAuthToken(token)
	return generated, err
}

func (t *authTokensStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	tokens, err := t.tokens.ListNamespaced(namespace.GlobalNamespace, v1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	for _, token := range tokens.Items {
		generated := transformToAuthToken(&token)
		result = append(result, generated)
	}
	return result, nil
}

func (t *authTokensStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	if err := t.tokens.DeleteNamespaced(namespace.GlobalNamespace, id, &v1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	return nil, nil
}

func transformToAuthToken(token *v3.SamlToken) map[string]interface{} {
	generated := map[string]interface{}{}
	if token == nil {
		return generated
	}
	generated["id"] = token.Name
	generated["type"] = "authToken"
	generated["name"] = token.Name
	generated["token"] = token.Token
	generated["expiresAt"] = token.ExpiresAt
	return generated
}
