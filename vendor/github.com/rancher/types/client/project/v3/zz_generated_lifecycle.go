package client

const (
	LifecycleType           = "lifecycle"
	LifecycleFieldPostStart = "postStart"
	LifecycleFieldPreStop   = "preStop"
)

type Lifecycle struct {
	PostStart *Handler `json:"postStart,omitempty"`
	PreStop   *Handler `json:"preStop,omitempty"`
}
