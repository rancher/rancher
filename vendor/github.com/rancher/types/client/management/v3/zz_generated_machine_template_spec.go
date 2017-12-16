package client

const (
	MachineTemplateSpecType                          = "machineTemplateSpec"
	MachineTemplateSpecFieldAuthCertificateAuthority = "authCertificateAuthority"
	MachineTemplateSpecFieldAuthKey                  = "authKey"
	MachineTemplateSpecFieldDescription              = "description"
	MachineTemplateSpecFieldDisplayName              = "displayName"
	MachineTemplateSpecFieldDockerVersion            = "dockerVersion"
	MachineTemplateSpecFieldDriver                   = "driver"
	MachineTemplateSpecFieldEngineEnv                = "engineEnv"
	MachineTemplateSpecFieldEngineInsecureRegistry   = "engineInsecureRegistry"
	MachineTemplateSpecFieldEngineInstallURL         = "engineInstallURL"
	MachineTemplateSpecFieldEngineLabel              = "engineLabel"
	MachineTemplateSpecFieldEngineOpt                = "engineOpt"
	MachineTemplateSpecFieldEngineRegistryMirror     = "engineRegistryMirror"
	MachineTemplateSpecFieldEngineStorageDriver      = "engineStorageDriver"
)

type MachineTemplateSpec struct {
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
}
