package client

const (
	StageType       = "stage"
	StageFieldName  = "name"
	StageFieldSteps = "steps"
	StageFieldWhen  = "when"
)

type Stage struct {
	Name  string       `json:"name,omitempty" yaml:"name,omitempty"`
	Steps []Step       `json:"steps,omitempty" yaml:"steps,omitempty"`
	When  *Constraints `json:"when,omitempty" yaml:"when,omitempty"`
}
