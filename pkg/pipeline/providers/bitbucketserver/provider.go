package bitbucketserver

import (
	"fmt"

	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	client "github.com/rancher/rancher/pkg/client/generated/project/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/pipeline/providers/common"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	schema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type BsProvider struct {
	common.BaseProvider
}

func (b *BsProvider) CustomizeSchemas(schemas *types.Schemas) {
	scpConfigBaseSchema := schemas.Schema(&schema.Version, client.SourceCodeProviderConfigType)
	configSchema := schemas.Schema(&schema.Version, client.BitbucketServerPipelineConfigType)
	configSchema.ActionHandler = b.ActionHandler
	configSchema.Formatter = b.Formatter
	configSchema.Store = subtype.NewSubTypeStore(client.BitbucketServerPipelineConfigType, scpConfigBaseSchema.Store)

	providerBaseSchema := schemas.Schema(&schema.Version, client.SourceCodeProviderType)
	providerSchema := schemas.Schema(&schema.Version, client.BitbucketServerProviderType)
	providerSchema.Formatter = b.providerFormatter
	providerSchema.ActionHandler = b.providerActionHandler
	providerSchema.Store = subtype.NewSubTypeStore(client.BitbucketServerProviderType, providerBaseSchema.Store)
}

func (b *BsProvider) GetName() string {
	return model.BitbucketServerType
}

func (b *BsProvider) TransformToSourceCodeProvider(config map[string]interface{}) map[string]interface{} {
	return b.BaseProvider.TransformToSourceCodeProvider(config, client.BitbucketServerProviderType)
}

func (b *BsProvider) GetProviderConfig(projectID string) (interface{}, error) {
	scpConfigObj, err := b.SourceCodeProviderConfigs.ObjectClient().UnstructuredClient().GetNamespaced(projectID, model.BitbucketServerType, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve BitbucketConfig, error: %v", err)
	}

	u, ok := scpConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve BitbucketConfig, cannot read k8s Unstructured data")
	}
	storedBitbucketPipelineConfigMap := u.UnstructuredContent()

	storedBitbucketPipelineConfig := &v32.BitbucketServerPipelineConfig{}
	if err := mapstructure.Decode(storedBitbucketPipelineConfigMap, storedBitbucketPipelineConfig); err != nil {
		return nil, fmt.Errorf("failed to decode the config, error: %v", err)
	}

	objectMeta, err := common.ObjectMetaFromUnstructureContent(storedBitbucketPipelineConfigMap)
	if err != nil {
		return nil, err
	}
	storedBitbucketPipelineConfig.ObjectMeta = *objectMeta
	storedBitbucketPipelineConfig.APIVersion = "project.cattle.io/v3"
	storedBitbucketPipelineConfig.Kind = v3.SourceCodeProviderConfigGroupVersionKind.Kind
	return storedBitbucketPipelineConfig, nil
}
