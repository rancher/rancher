package client

const (
	QuerySpecType                = "querySpec"
	QuerySpecFieldLookbackDelta  = "lookbackDelta"
	QuerySpecFieldMaxConcurrency = "maxConcurrency"
	QuerySpecFieldMaxSamples     = "maxSamples"
	QuerySpecFieldTimeout        = "timeout"
)

type QuerySpec struct {
	LookbackDelta  string `json:"lookbackDelta,omitempty" yaml:"lookbackDelta,omitempty"`
	MaxConcurrency *int64 `json:"maxConcurrency,omitempty" yaml:"maxConcurrency,omitempty"`
	MaxSamples     *int64 `json:"maxSamples,omitempty" yaml:"maxSamples,omitempty"`
	Timeout        string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}
