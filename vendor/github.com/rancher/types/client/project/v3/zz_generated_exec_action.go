package client

const (
	ExecActionType         = "execAction"
	ExecActionFieldCommand = "command"
)

type ExecAction struct {
	Command []string `json:"command,omitempty"`
}
