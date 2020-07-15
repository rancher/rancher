package client

const (
	ComposeSpecType                = "composeSpec"
	ComposeSpecFieldRancherCompose = "rancherCompose"
)

type ComposeSpec struct {
	RancherCompose string `json:"rancherCompose,omitempty" yaml:"rancherCompose,omitempty"`
}
