package nodetemplates

import (
	"maps"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"gopkg.in/yaml.v2"
	"k8s.io/utils/strings/slices"
)

// The json/yaml config key for the node template config
const NodeTemplateConfigurationFileKey = "nodeTemplate"

// NodeTemplate is the main struct needed to create a node template for an RKE1 cluster
type NodeTemplate struct {
	types.Resource
	Annotations                     map[string]string                `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AuthCertificateAuthority        string                           `json:"authCertificateAuthority,omitempty" yaml:"authCertificateAuthority,omitempty"`
	AuthKey                         string                           `json:"authKey,omitempty" yaml:"authKey,omitempty"`
	CloudCredentialID               string                           `json:"cloudCredentialId,omitempty" yaml:"cloudCredentialId,omitempty"`
	Created                         string                           `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                       string                           `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description                     string                           `json:"description,omitempty" yaml:"description,omitempty"`
	DockerVersion                   string                           `json:"dockerVersion,omitempty" yaml:"dockerVersion,omitempty"`
	Driver                          string                           `json:"driver,omitempty" yaml:"driver,omitempty"`
	EngineEnv                       map[string]string                `json:"engineEnv,omitempty" yaml:"engineEnv,omitempty"`
	EngineInsecureRegistry          []string                         `json:"engineInsecureRegistry,omitempty" yaml:"engineInsecureRegistry,omitempty"`
	EngineInstallURL                string                           `json:"engineInstallURL,omitempty" yaml:"engineInstallURL,omitempty"`
	EngineLabel                     map[string]string                `json:"engineLabel,omitempty" yaml:"engineLabel,omitempty"`
	EngineOpt                       map[string]string                `json:"engineOpt,omitempty" yaml:"engineOpt,omitempty"`
	EngineRegistryMirror            []string                         `json:"engineRegistryMirror,omitempty" yaml:"engineRegistryMirror,omitempty"`
	EngineStorageDriver             string                           `json:"engineStorageDriver,omitempty" yaml:"engineStorageDriver,omitempty"`
	Label                           map[string]string                `json:"label,omitempty" yaml:"label,omitempty"`
	AmazonEC2NodeTemplateConfig     *AmazonEC2NodeTemplateConfig     `json:"amazonec2Config" yaml:"amazonec2Config,omitempty"`
	AzureNodeTemplateConfig         *AzureNodeTemplateConfig         `json:"azureConfig" yaml:"azureConfig,omitempty"`
	HarvesterNodeTemplateConfig     *HarvesterNodeTemplateConfig     `json:"harvesterConfig" yaml:"harvesterConfig,omitempty"`
	LinodeNodeTemplateConfig        *LinodeNodeTemplateConfig        `json:"linodeConfig" yaml:"linodeConfig,omitempty"`
	VmwareVsphereNodeTemplateConfig *VmwareVsphereNodeTemplateConfig `json:"vmwarevsphereConfig" yaml:"vmwarevsphereConfig,omitempty"`
	Name                            string                           `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceID                     string                           `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	Removed                         string                           `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                           string                           `json:"state,omitempty" yaml:"state,omitempty"`
	Transitioning                   string                           `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage            string                           `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	Type                            string                           `json:"type,omitempty" yaml:"type,omitempty"`
	UUID                            string                           `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	UseInternalIPAddress            *bool                            `json:"useInternalIpAddress,omitempty" yaml:"useInternalIpAddress,omitempty"`
}

func providerTemplateConfigKeys() []string {
	return []string{
		AmazonEC2NodeTemplateConfigurationFileKey,
		AzureNodeTemplateConfigurationFileKey,
		HarvesterNodeTemplateConfigurationFileKey,
		LinodeNodeTemplateConfigurationFileKey,
		VmwareVsphereNodeTemplateConfigurationFileKey,
	}
}

// MergeOverride merges two NodeTemplate objects by overriding fields from n1 with fields from n2
//   - preserves fields not present in n2
//   - deletes all provider keys except the specified providerTemplateConfigKey from both NodeTemplate objects before merging
//
// providerTemplateConfigKey: The key representing the provider template configuration to preserve during merging
//
// returns a pointer to the merged NodeTemplate and an error if any
func (n1 *NodeTemplate) MergeOverride(n2 *NodeTemplate, providerTemplateConfigKey string) (*NodeTemplate, error) {
	var n1Data map[string]any
	n1YAML, err := yaml.Marshal(&n1)
	if err != nil {
		return nil, errors.Wrap(err, "MergeOverride: ")
	}
	err = yaml.Unmarshal(n1YAML, &n1Data)
	if err != nil {
		return nil, errors.Wrap(err, "MergeOverride: ")
	}

	var n2Data map[string]any
	n2YAML, err := yaml.Marshal(&n2)
	if err != nil {
		return nil, errors.Wrap(err, "MergeOverride: ")
	}
	err = yaml.Unmarshal(n2YAML, &n2Data)
	if err != nil {
		return nil, errors.Wrap(err, "MergeOverride: ")
	}

	configKeys := providerTemplateConfigKeys()
	var keyPosition int
	for pos, configKey := range configKeys {
		if configKey == providerTemplateConfigKey {
			keyPosition = pos
		}
	}

	// Delete all other provider keys from both nodetemplates
	keysToDelete := append(configKeys[:keyPosition], configKeys[keyPosition+1:]...)
	maps.DeleteFunc(n1Data, func(k string, v any) bool {
		return slices.Contains(keysToDelete, k) && k != providerTemplateConfigKey
	})
	maps.DeleteFunc(n2Data, func(k string, v any) bool {
		return slices.Contains(keysToDelete, k) && k != providerTemplateConfigKey
	})

	maps.Copy(n1Data, n2Data)

	tempData, err := yaml.Marshal(n1Data)
	if err != nil {
		return nil, errors.Wrap(err, "MergeOverride: ")
	}

	var mergedNodeTemplate = NodeTemplate{}
	err = yaml.Unmarshal(tempData, &mergedNodeTemplate)
	if err != nil {
		return nil, errors.Wrap(err, "MergeOverride: ")
	}
	return &mergedNodeTemplate, nil
}
