package client

const (
	InjectDelayType                  = "injectDelay"
	InjectDelayFieldExponentialDelay = "exponentialDelay"
	InjectDelayFieldFixedDelay       = "fixedDelay"
	InjectDelayFieldPercent          = "percent"
)

type InjectDelay struct {
	ExponentialDelay string `json:"exponentialDelay,omitempty" yaml:"exponentialDelay,omitempty"`
	FixedDelay       string `json:"fixedDelay,omitempty" yaml:"fixedDelay,omitempty"`
	Percent          int64  `json:"percent,omitempty" yaml:"percent,omitempty"`
}
