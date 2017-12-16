package client

const (
	MachineSpecType                          = "machineSpec"
	MachineSpecFieldAmazonEC2Config          = "amazonEc2Config"
	MachineSpecFieldAuthCertificateAuthority = "authCertificateAuthority"
	MachineSpecFieldAuthKey                  = "authKey"
	MachineSpecFieldAzureConfig              = "azureConfig"
	MachineSpecFieldClusterId                = "clusterId"
	MachineSpecFieldConfigSource             = "configSource"
	MachineSpecFieldDescription              = "description"
	MachineSpecFieldDigitalOceanConfig       = "digitalOceanConfig"
	MachineSpecFieldDockerVersion            = "dockerVersion"
	MachineSpecFieldDriver                   = "driver"
	MachineSpecFieldEngineEnv                = "engineEnv"
	MachineSpecFieldEngineInsecureRegistry   = "engineInsecureRegistry"
	MachineSpecFieldEngineInstallURL         = "engineInstallURL"
	MachineSpecFieldEngineLabel              = "engineLabel"
	MachineSpecFieldEngineOpt                = "engineOpt"
	MachineSpecFieldEngineRegistryMirror     = "engineRegistryMirror"
	MachineSpecFieldEngineStorageDriver      = "engineStorageDriver"
	MachineSpecFieldExternalId               = "externalId"
	MachineSpecFieldMachineTemplateId        = "machineTemplateId"
	MachineSpecFieldPodCIDR                  = "podCIDR"
	MachineSpecFieldProviderID               = "providerID"
	MachineSpecFieldRole                     = "role"
	MachineSpecFieldTaints                   = "taints"
	MachineSpecFieldUnschedulable            = "unschedulable"
)

type MachineSpec struct {
	AmazonEC2Config          *AmazonEC2Config    `json:"amazonEc2Config,omitempty"`
	AuthCertificateAuthority string              `json:"authCertificateAuthority,omitempty"`
	AuthKey                  string              `json:"authKey,omitempty"`
	AzureConfig              *AzureConfig        `json:"azureConfig,omitempty"`
	ClusterId                string              `json:"clusterId,omitempty"`
	ConfigSource             *NodeConfigSource   `json:"configSource,omitempty"`
	Description              string              `json:"description,omitempty"`
	DigitalOceanConfig       *DigitalOceanConfig `json:"digitalOceanConfig,omitempty"`
	DockerVersion            string              `json:"dockerVersion,omitempty"`
	Driver                   string              `json:"driver,omitempty"`
	EngineEnv                map[string]string   `json:"engineEnv,omitempty"`
	EngineInsecureRegistry   []string            `json:"engineInsecureRegistry,omitempty"`
	EngineInstallURL         string              `json:"engineInstallURL,omitempty"`
	EngineLabel              map[string]string   `json:"engineLabel,omitempty"`
	EngineOpt                map[string]string   `json:"engineOpt,omitempty"`
	EngineRegistryMirror     []string            `json:"engineRegistryMirror,omitempty"`
	EngineStorageDriver      string              `json:"engineStorageDriver,omitempty"`
	ExternalId               string              `json:"externalId,omitempty"`
	MachineTemplateId        string              `json:"machineTemplateId,omitempty"`
	PodCIDR                  string              `json:"podCIDR,omitempty"`
	ProviderID               string              `json:"providerID,omitempty"`
	Role                     string              `json:"role,omitempty"`
	Taints                   []Taint             `json:"taints,omitempty"`
	Unschedulable            *bool               `json:"unschedulable,omitempty"`
}
