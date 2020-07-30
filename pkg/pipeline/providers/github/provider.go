package github

import (
	"fmt"

	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	client "github.com/rancher/rancher/pkg/client/generated/project/v3"
	mv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/pipeline/providers/common"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	schema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type GhProvider struct {
	common.BaseProvider
	AuthConfigs mv3.AuthConfigInterface
}

func (g *GhProvider) CustomizeSchemas(schemas *types.Schemas) {
	scpConfigBaseSchema := schemas.Schema(&schema.Version, client.SourceCodeProviderConfigType)
	configSchema := schemas.Schema(&schema.Version, client.GithubPipelineConfigType)
	configSchema.ActionHandler = g.ActionHandler
	configSchema.Formatter = g.Formatter
	configSchema.Store = subtype.NewSubTypeStore(client.GithubPipelineConfigType, scpConfigBaseSchema.Store)

	providerBaseSchema := schemas.Schema(&schema.Version, client.SourceCodeProviderType)
	providerSchema := schemas.Schema(&schema.Version, client.GithubProviderType)
	providerSchema.Formatter = g.providerFormatter
	providerSchema.ActionHandler = g.providerActionHandler
	providerSchema.Store = subtype.NewSubTypeStore(client.GithubProviderType, providerBaseSchema.Store)
}

func (g *GhProvider) GetName() string {
	return model.GithubType
}

func (g *GhProvider) TransformToSourceCodeProvider(config map[string]interface{}) map[string]interface{} {
	m := g.BaseProvider.TransformToSourceCodeProvider(config, client.GithubProviderType)
	m[client.GithubProviderFieldRedirectURL] = formGithubRedirectURLFromMap(config)
	return m
}

func (g *GhProvider) GetProviderConfig(projectID string) (interface{}, error) {
	scpConfigObj, err := g.SourceCodeProviderConfigs.ObjectClient().UnstructuredClient().GetNamespaced(projectID, model.GithubType, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve GithubConfig, error: %v", err)
	}

	u, ok := scpConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve GithubConfig, cannot read k8s Unstructured data")
	}
	storedGithubPipelineConfigMap := u.UnstructuredContent()

	storedGithubPipelineConfig := &v32.GithubPipelineConfig{}
	if err := mapstructure.Decode(storedGithubPipelineConfigMap, storedGithubPipelineConfig); err != nil {
		return nil, fmt.Errorf("failed to decode the config, error: %v", err)
	}

	if storedGithubPipelineConfig.Inherit {
		globalConfig, err := g.getGithubConfigCR()
		if err != nil {
			return nil, err
		}
		storedGithubPipelineConfig.ClientSecret = globalConfig.ClientSecret
	}

	objectMeta, err := common.ObjectMetaFromUnstructureContent(storedGithubPipelineConfigMap)
	if err != nil {
		return nil, err
	}
	storedGithubPipelineConfig.ObjectMeta = *objectMeta
	storedGithubPipelineConfig.APIVersion = "project.cattle.io/v3"
	storedGithubPipelineConfig.Kind = v3.SourceCodeProviderConfigGroupVersionKind.Kind
	return storedGithubPipelineConfig, nil
}
