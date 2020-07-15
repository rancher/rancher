package bitbucketcloud

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

type BcProvider struct {
	common.BaseProvider
}

func (b *BcProvider) CustomizeSchemas(schemas *types.Schemas) {
	scpConfigBaseSchema := schemas.Schema(&schema.Version, client.SourceCodeProviderConfigType)
	configSchema := schemas.Schema(&schema.Version, client.BitbucketCloudPipelineConfigType)
	configSchema.ActionHandler = b.ActionHandler
	configSchema.Formatter = b.Formatter
	configSchema.Store = subtype.NewSubTypeStore(client.BitbucketCloudPipelineConfigType, scpConfigBaseSchema.Store)

	providerBaseSchema := schemas.Schema(&schema.Version, client.SourceCodeProviderType)
	providerSchema := schemas.Schema(&schema.Version, client.BitbucketCloudProviderType)
	providerSchema.Formatter = b.providerFormatter
	providerSchema.ActionHandler = b.providerActionHandler
	providerSchema.Store = subtype.NewSubTypeStore(client.BitbucketCloudProviderType, providerBaseSchema.Store)
}

func (b *BcProvider) GetName() string {
	return model.BitbucketCloudType
}

func (b *BcProvider) TransformToSourceCodeProvider(config map[string]interface{}) map[string]interface{} {
	m := b.BaseProvider.TransformToSourceCodeProvider(config, client.BitbucketCloudProviderType)
	m[client.BitbucketCloudProviderFieldRedirectURL] = formBitbucketRedirectURLFromMap(config)
	return m
}

func (b *BcProvider) GetProviderConfig(projectID string) (interface{}, error) {
	scpConfigObj, err := b.SourceCodeProviderConfigs.ObjectClient().UnstructuredClient().GetNamespaced(projectID, model.BitbucketCloudType, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve BitbucketConfig, error: %v", err)
	}

	u, ok := scpConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve BitbucketConfig, cannot read k8s Unstructured data")
	}
	storedBitbucketPipelineConfigMap := u.UnstructuredContent()

	storedBitbucketPipelineConfig := &v32.BitbucketCloudPipelineConfig{}
	if err := mapstructure.Decode(storedBitbucketPipelineConfigMap, storedBitbucketPipelineConfig); err != nil {
		return nil, fmt.Errorf("failed to decode the config, error: %v", err)
	}

	metadataMap, ok := storedBitbucketPipelineConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to retrieve BitbucketConfig metadata, cannot read k8s Unstructured data")
	}

	typemeta := &metav1.ObjectMeta{}
	//time.Time cannot decode directly
	delete(metadataMap, "creationTimestamp")
	if err := mapstructure.Decode(metadataMap, typemeta); err != nil {
		return nil, fmt.Errorf("failed to decode the config, error: %v", err)
	}
	storedBitbucketPipelineConfig.ObjectMeta = *typemeta
	storedBitbucketPipelineConfig.APIVersion = "project.cattle.io/v3"
	storedBitbucketPipelineConfig.Kind = v3.SourceCodeProviderConfigGroupVersionKind.Kind
	return storedBitbucketPipelineConfig, nil
}
