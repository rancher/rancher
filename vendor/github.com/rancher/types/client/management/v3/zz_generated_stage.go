package client

const (
	StageType       = "stage"
	StageFieldName  = "name"
	StageFieldSteps = "steps"
)

type Stage struct {
	Name  string `json:"name,omitempty"`
	Steps []Step `json:"steps,omitempty"`
}
