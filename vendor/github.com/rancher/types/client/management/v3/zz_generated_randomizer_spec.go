package client

const (
	RandomizerSpecType               = "randomizerSpec"
	RandomizerSpecFieldExampleString = "rancherCompose"
)

type RandomizerSpec struct {
	ExampleString string `json:"rancherCompose,omitempty" yaml:"rancherCompose,omitempty"`
}
