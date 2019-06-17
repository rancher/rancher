package client

const (
	HTTPRetryType               = "httpRetry"
	HTTPRetryFieldAttempts      = "attempts"
	HTTPRetryFieldPerTryTimeout = "perTryTimeout"
)

type HTTPRetry struct {
	Attempts      int64  `json:"attempts,omitempty" yaml:"attempts,omitempty"`
	PerTryTimeout string `json:"perTryTimeout,omitempty" yaml:"perTryTimeout,omitempty"`
}
