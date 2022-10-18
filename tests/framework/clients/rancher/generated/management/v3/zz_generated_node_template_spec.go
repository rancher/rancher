package client

const (
	NodeTemplateSpecType                          = "nodeTemplateSpec"
	NodeTemplateSpecFieldAuthCertificateAuthority = "authCertificateAuthority"
	NodeTemplateSpecFieldAuthKey                  = "authKey"
	NodeTemplateSpecFieldCloudCredentialID        = "cloudCredentialId"
	NodeTemplateSpecFieldDescription              = "description"
	NodeTemplateSpecFieldDisplayName              = "displayName"
	NodeTemplateSpecFieldDockerVersion            = "dockerVersion"
	NodeTemplateSpecFieldDriver                   = "driver"
	NodeTemplateSpecFieldEngineEnv                = "engineEnv"
	NodeTemplateSpecFieldEngineInsecureRegistry   = "engineInsecureRegistry"
	NodeTemplateSpecFieldEngineInstallURL         = "engineInstallURL"
	NodeTemplateSpecFieldEngineLabel              = "engineLabel"
	NodeTemplateSpecFieldEngineOpt                = "engineOpt"
	NodeTemplateSpecFieldEngineRegistryMirror     = "engineRegistryMirror"
	NodeTemplateSpecFieldEngineStorageDriver      = "engineStorageDriver"
	NodeTemplateSpecFieldLogOpt                   = "logOpt"
	NodeTemplateSpecFieldNodeTaints               = "nodeTaints"
	NodeTemplateSpecFieldStorageOpt               = "storageOpt"
	NodeTemplateSpecFieldUseInternalIPAddress     = "useInternalIpAddress"
)

type NodeTemplateSpec struct {
	AuthCertificateAuthority string            `json:"authCertificateAuthority,omitempty" yaml:"authCertificateAuthority,omitempty"`
	AuthKey                  string            `json:"authKey,omitempty" yaml:"authKey,omitempty"`
	CloudCredentialID        string            `json:"cloudCredentialId,omitempty" yaml:"cloudCredentialId,omitempty"`
	Description              string            `json:"description,omitempty" yaml:"description,omitempty"`
	DisplayName              string            `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	DockerVersion            string            `json:"dockerVersion,omitempty" yaml:"dockerVersion,omitempty"`
	Driver                   string            `json:"driver,omitempty" yaml:"driver,omitempty"`
	EngineEnv                map[string]string `json:"engineEnv,omitempty" yaml:"engineEnv,omitempty"`
	EngineInsecureRegistry   []string          `json:"engineInsecureRegistry,omitempty" yaml:"engineInsecureRegistry,omitempty"`
	EngineInstallURL         string            `json:"engineInstallURL,omitempty" yaml:"engineInstallURL,omitempty"`
	EngineLabel              map[string]string `json:"engineLabel,omitempty" yaml:"engineLabel,omitempty"`
	EngineOpt                map[string]string `json:"engineOpt,omitempty" yaml:"engineOpt,omitempty"`
	EngineRegistryMirror     []string          `json:"engineRegistryMirror,omitempty" yaml:"engineRegistryMirror,omitempty"`
	EngineStorageDriver      string            `json:"engineStorageDriver,omitempty" yaml:"engineStorageDriver,omitempty"`
	LogOpt                   map[string]string `json:"logOpt,omitempty" yaml:"logOpt,omitempty"`
	NodeTaints               []Taint           `json:"nodeTaints,omitempty" yaml:"nodeTaints,omitempty"`
	StorageOpt               map[string]string `json:"storageOpt,omitempty" yaml:"storageOpt,omitempty"`
	UseInternalIPAddress     *bool             `json:"useInternalIpAddress,omitempty" yaml:"useInternalIpAddress,omitempty"`
}
