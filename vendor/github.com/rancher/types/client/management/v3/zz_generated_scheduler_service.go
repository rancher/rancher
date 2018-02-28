package client

const (
	SchedulerServiceType           = "schedulerService"
	SchedulerServiceFieldExtraArgs = "extraArgs"
	SchedulerServiceFieldImage     = "image"
)

type SchedulerService struct {
	ExtraArgs map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	Image     string            `json:"image,omitempty" yaml:"image,omitempty"`
}
