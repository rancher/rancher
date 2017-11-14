package client

const (
	ContainerStateTerminatedType             = "containerStateTerminated"
	ContainerStateTerminatedFieldContainerID = "containerID"
	ContainerStateTerminatedFieldExitCode    = "exitCode"
	ContainerStateTerminatedFieldFinishedAt  = "finishedAt"
	ContainerStateTerminatedFieldMessage     = "message"
	ContainerStateTerminatedFieldReason      = "reason"
	ContainerStateTerminatedFieldSignal      = "signal"
	ContainerStateTerminatedFieldStartedAt   = "startedAt"
)

type ContainerStateTerminated struct {
	ContainerID string `json:"containerID,omitempty"`
	ExitCode    *int64 `json:"exitCode,omitempty"`
	FinishedAt  string `json:"finishedAt,omitempty"`
	Message     string `json:"message,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Signal      *int64 `json:"signal,omitempty"`
	StartedAt   string `json:"startedAt,omitempty"`
}
