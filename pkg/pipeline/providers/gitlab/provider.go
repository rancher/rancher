package gitlab

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/pipeline/providers/common"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	v3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	client "github.com/rancher/types/client/project/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type GlProvider struct {
	common.BaseProvider
}

func (g *GlProvider) CustomizeSchemas(schemas *types.Schemas) {
	scpConfigBaseSchema := schemas.Schema(&schema.Version, client.SourceCodeProviderConfigType)
	configSchema := schemas.Schema(&schema.Version, client.GitlabPipelineConfigType)
	configSchema.ActionHandler = g.ActionHandler
	configSchema.Formatter = g.Formatter
	configSchema.Store = subtype.NewSubTypeStore(client.GitlabPipelineConfigType, scpConfigBaseSchema.Store)

	providerBaseSchema := schemas.Schema(&schema.Version, client.SourceCodeProviderType)
	providerSchema := schemas.Schema(&schema.Version, client.GitlabProviderType)
	providerSchema.Formatter = g.providerFormatter
	providerSchema.ActionHandler = g.providerActionHandler
	providerSchema.Store = subtype.NewSubTypeStore(client.GitlabProviderType, providerBaseSchema.Store)
}

func (g *GlProvider) GetName() string {
	return model.GitlabType
}

func (g *GlProvider) TransformToSourceCodeProvider(config map[string]interface{}) map[string]interface{} {
	m := g.BaseProvider.TransformToSourceCodeProvider(config, client.GitlabProviderType)
	m[client.GitlabProviderFieldRedirectURL] = formGitlabRedirectURLFromMap(config)
	return m
}

func (g *GlProvider) GetProviderConfig(projectID string) (interface{}, error) {
	scpConfigObj, err := g.SourceCodeProviderConfigs.ObjectClient().UnstructuredClient().GetNamespaced(projectID, model.GitlabType, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve GitlabConfig, error: %v", err)
	}

	u, ok := scpConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve GitlabConfig, cannot read k8s Unstructured data")
	}
	storedGitlabPipelineConfigMap := u.UnstructuredContent()

	storedGitlabPipelineConfig := &v3.GitlabPipelineConfig{}
	if err := mapstructure.Decode(storedGitlabPipelineConfigMap, storedGitlabPipelineConfig); err != nil {
		return nil, fmt.Errorf("failed to decode the config, error: %v", err)
	}

	objectMeta, err := common.ObjectMetaFromUnstructureContent(storedGitlabPipelineConfigMap)
	if err != nil {
		return nil, err
	}
	storedGitlabPipelineConfig.ObjectMeta = *objectMeta
	storedGitlabPipelineConfig.APIVersion = "project.cattle.io/v3"
	storedGitlabPipelineConfig.Kind = v3.SourceCodeProviderConfigGroupVersionKind.Kind
	return storedGitlabPipelineConfig, nil
}

func formGitlabRedirectURLFromMap(config map[string]interface{}) string {
	hostname := convert.ToString(config[client.GitlabPipelineConfigFieldHostname])
	clientID := convert.ToString(config[client.GitlabPipelineConfigFieldClientID])
	tls := convert.ToBool(config[client.GitlabPipelineConfigFieldTLS])
	return gitlabRedirectURL(hostname, clientID, tls)
}

func gitlabRedirectURL(hostname, clientID string, tls bool) string {
	redirect := ""
	if hostname != "" {
		scheme := "http://"
		if tls {
			scheme = "https://"
		}
		redirect = scheme + hostname
	} else {
		redirect = gitlabDefaultHostName
	}
	return fmt.Sprintf("%s/oauth/authorize?client_id=%s&response_type=code", redirect, clientID)
}
