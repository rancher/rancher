package client

const (
	LifecycleType            = "lifecycle"
	LifecycleFieldPostStart  = "postStart"
	LifecycleFieldPreStop    = "preStop"
	LifecycleFieldStopSignal = "stopSignal"
)

type Lifecycle struct {
	PostStart  *LifecycleHandler `json:"postStart,omitempty" yaml:"postStart,omitempty"`
	PreStop    *LifecycleHandler `json:"preStop,omitempty" yaml:"preStop,omitempty"`
	StopSignal string            `json:"stopSignal,omitempty" yaml:"stopSignal,omitempty"`
}
