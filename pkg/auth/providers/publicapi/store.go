package publicapi

import (
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/github"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
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
	if id == "github" {
		gh, err := s.getGithub()
		if err != nil {
			return nil, err
		}

		if gh != nil {
			return gh, nil
		}
		return nil, httperror.NewAPIError(httperror.NotFound, "")
	}

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

	// TODO add a framework for providers specifying their own output customizations
	gh, err := s.getGithub()
	if err != nil {
		return nil, err
	}
	if gh != nil {
		result = append(result, gh)
	}

	for _, ac := range configs.Items {
		if ac.Enabled && ac.Name != "github" {
			//if ac.Enabled && ac.Name != "github" {
			result = append(result, map[string]interface{}{"id": ac.Name, "type": strings.Replace(ac.Type, "Config", "Provider", -1)})
		}
	}
	return result, nil
}

func (s *authProvidersStore) getGithub() (map[string]interface{}, error) {
	o, err := s.mgmt.Management.AuthConfigs("").ObjectClient().UnstructuredClient().Get("github", v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	u, _ := o.(runtime.Unstructured)
	gh := &v3.GithubConfig{}
	mapstructure.Decode(u.UnstructuredContent(), gh)

	if gh.Enabled {
		redirectURL := github.FormGithubRedirectURL(gh)
		return map[string]interface{}{
			"id":          "github",
			"type":        "githubProvider",
			"redirectUrl": redirectURL,
		}, nil
	}
	return nil, nil
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
