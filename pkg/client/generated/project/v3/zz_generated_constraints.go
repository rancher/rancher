package client

const (
	ConstraintsType        = "constraints"
	ConstraintsFieldBranch = "branch"
	ConstraintsFieldEvent  = "event"
)

type Constraints struct {
	Branch *Constraint `json:"branch,omitempty" yaml:"branch,omitempty"`
	Event  *Constraint `json:"event,omitempty" yaml:"event,omitempty"`
}
