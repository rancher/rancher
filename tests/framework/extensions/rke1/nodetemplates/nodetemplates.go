package nodetemplates

import (
	"github.com/rancher/norman/types"
)

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
