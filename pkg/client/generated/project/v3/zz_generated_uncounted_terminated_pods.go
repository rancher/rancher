package client

const (
	UncountedTerminatedPodsType           = "uncountedTerminatedPods"
	UncountedTerminatedPodsFieldFailed    = "failed"
	UncountedTerminatedPodsFieldSucceeded = "succeeded"
)

type UncountedTerminatedPods struct {
	Failed    []string `json:"failed,omitempty" yaml:"failed,omitempty"`
	Succeeded []string `json:"succeeded,omitempty" yaml:"succeeded,omitempty"`
}
