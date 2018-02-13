package client

const (
	NodeTemplateSpecType                          = "nodeTemplateSpec"
	NodeTemplateSpecFieldAuthCertificateAuthority = "authCertificateAuthority"
	NodeTemplateSpecFieldAuthKey                  = "authKey"
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
	NodeTemplateSpecFieldUseInternalIPAddress     = "useInternalIpAddress"
)

type NodeTemplateSpec struct {
	AuthCertificateAuthority string            `json:"authCertificateAuthority,omitempty"`
	AuthKey                  string            `json:"authKey,omitempty"`
	Description              string            `json:"description,omitempty"`
	DisplayName              string            `json:"displayName,omitempty"`
	DockerVersion            string            `json:"dockerVersion,omitempty"`
	Driver                   string            `json:"driver,omitempty"`
	EngineEnv                map[string]string `json:"engineEnv,omitempty"`
	EngineInsecureRegistry   []string          `json:"engineInsecureRegistry,omitempty"`
	EngineInstallURL         string            `json:"engineInstallURL,omitempty"`
	EngineLabel              map[string]string `json:"engineLabel,omitempty"`
	EngineOpt                map[string]string `json:"engineOpt,omitempty"`
	EngineRegistryMirror     []string          `json:"engineRegistryMirror,omitempty"`
	EngineStorageDriver      string            `json:"engineStorageDriver,omitempty"`
	UseInternalIPAddress     *bool             `json:"useInternalIpAddress,omitempty"`
}
