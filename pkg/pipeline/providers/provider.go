package providers

import (
	"fmt"
	"github.com/rancher/norman/types"
)

type SourceCodeProvider interface {
	GetName() string
	CustomizeSchemas(schemas *types.Schemas)
	TransformToSourceCodeProvider(sourceCodeProviderConfig map[string]interface{}) map[string]interface{}
	GetProviderConfig(projectID string) (interface{}, error)
}

func GetProvider(providerName string) (SourceCodeProvider, error) {
	if provider, ok := providers[providerName]; ok {
		if provider != nil {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("No such provider '%s'", providerName)
}

func GetSourceCodeProviderConfig(pType string, projectID string) (interface{}, error) {
	provider, err := GetProvider(pType)
	if err != nil {
		return nil, err
	}
	return provider.GetProviderConfig(projectID)
}
