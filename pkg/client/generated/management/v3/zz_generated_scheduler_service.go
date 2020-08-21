package client

const (
	SchedulerServiceType                   = "schedulerService"
	SchedulerServiceFieldExtraArgs         = "extraArgs"
	SchedulerServiceFieldExtraBinds        = "extraBinds"
	SchedulerServiceFieldExtraEnv          = "extraEnv"
	SchedulerServiceFieldImage             = "image"
	SchedulerServiceFieldWindowsExtraArgs  = "winExtraArgs"
	SchedulerServiceFieldWindowsExtraBinds = "winExtraBinds"
	SchedulerServiceFieldWindowsExtraEnv   = "winExtraEnv"
)

type SchedulerService struct {
	ExtraArgs         map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ExtraBinds        []string          `json:"extraBinds,omitempty" yaml:"extraBinds,omitempty"`
	ExtraEnv          []string          `json:"extraEnv,omitempty" yaml:"extraEnv,omitempty"`
	Image             string            `json:"image,omitempty" yaml:"image,omitempty"`
	WindowsExtraArgs  map[string]string `json:"winExtraArgs,omitempty" yaml:"winExtraArgs,omitempty"`
	WindowsExtraBinds []string          `json:"winExtraBinds,omitempty" yaml:"winExtraBinds,omitempty"`
	WindowsExtraEnv   []string          `json:"winExtraEnv,omitempty" yaml:"winExtraEnv,omitempty"`
}
