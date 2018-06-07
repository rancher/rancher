package client

const (
	SchedulerServiceType            = "schedulerService"
	SchedulerServiceFieldExtraArgs  = "extraArgs"
	SchedulerServiceFieldExtraBinds = "extraBinds"
	SchedulerServiceFieldExtraEnv   = "extraEnv"
	SchedulerServiceFieldImage      = "image"
)

type SchedulerService struct {
	ExtraArgs  map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ExtraBinds []string          `json:"extraBinds,omitempty" yaml:"extraBinds,omitempty"`
	ExtraEnv   []string          `json:"extraEnv,omitempty" yaml:"extraEnv,omitempty"`
	Image      string            `json:"image,omitempty" yaml:"image,omitempty"`
}
