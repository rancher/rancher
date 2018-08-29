package client

const (
	ConstraintType         = "constraint"
	ConstraintFieldExclude = "exclude"
	ConstraintFieldInclude = "include"
)

type Constraint struct {
	Exclude []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
	Include []string `json:"include,omitempty" yaml:"include,omitempty"`
}
