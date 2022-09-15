package client

const (
	LifecycleType           = "lifecycle"
	LifecycleFieldPostStart = "postStart"
	LifecycleFieldPreStop   = "preStop"
)

type Lifecycle struct {
	PostStart *LifecycleHandler `json:"postStart,omitempty" yaml:"postStart,omitempty"`
	PreStop   *LifecycleHandler `json:"preStop,omitempty" yaml:"preStop,omitempty"`
}
