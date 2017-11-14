package client

const (
	ActionType        = "action"
	ActionFieldInput  = "input"
	ActionFieldOutput = "output"
)

type Action struct {
	Input  string `json:"input,omitempty"`
	Output string `json:"output,omitempty"`
}
