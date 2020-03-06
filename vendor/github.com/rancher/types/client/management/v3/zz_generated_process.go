package client

const (
	ProcessType                         = "process"
	ProcessFieldArgs                    = "args"
	ProcessFieldBinds                   = "binds"
	ProcessFieldCommand                 = "command"
	ProcessFieldEnv                     = "env"
	ProcessFieldHealthCheck             = "healthCheck"
	ProcessFieldImage                   = "image"
	ProcessFieldImageRegistryAuthConfig = "imageRegistryAuthConfig"
	ProcessFieldLabels                  = "labels"
	ProcessFieldName                    = "name"
	ProcessFieldNetworkMode             = "networkMode"
	ProcessFieldPidMode                 = "pidMode"
	ProcessFieldPrivileged              = "privileged"
	ProcessFieldPublish                 = "publish"
	ProcessFieldRestartPolicy           = "restartPolicy"
	ProcessFieldUser                    = "user"
	ProcessFieldVolumesFrom             = "volumesFrom"
)

type Process struct {
	Args                    []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Binds                   []string          `json:"binds,omitempty" yaml:"binds,omitempty"`
	Command                 []string          `json:"command,omitempty" yaml:"command,omitempty"`
	Env                     []string          `json:"env,omitempty" yaml:"env,omitempty"`
	HealthCheck             *HealthCheck      `json:"healthCheck,omitempty" yaml:"healthCheck,omitempty"`
	Image                   string            `json:"image,omitempty" yaml:"image,omitempty"`
	ImageRegistryAuthConfig string            `json:"imageRegistryAuthConfig,omitempty" yaml:"imageRegistryAuthConfig,omitempty"`
	Labels                  map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                    string            `json:"name,omitempty" yaml:"name,omitempty"`
	NetworkMode             string            `json:"networkMode,omitempty" yaml:"networkMode,omitempty"`
	PidMode                 string            `json:"pidMode,omitempty" yaml:"pidMode,omitempty"`
	Privileged              bool              `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	Publish                 []string          `json:"publish,omitempty" yaml:"publish,omitempty"`
	RestartPolicy           string            `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	User                    string            `json:"user,omitempty" yaml:"user,omitempty"`
	VolumesFrom             []string          `json:"volumesFrom,omitempty" yaml:"volumesFrom,omitempty"`
}
