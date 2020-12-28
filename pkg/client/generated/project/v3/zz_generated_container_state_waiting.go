package client

const (
	ContainerStateWaitingType         = "containerStateWaiting"
	ContainerStateWaitingFieldMessage = "message"
	ContainerStateWaitingFieldReason  = "reason"
)

type ContainerStateWaiting struct {
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason  string `json:"reason,omitempty" yaml:"reason,omitempty"`
}
