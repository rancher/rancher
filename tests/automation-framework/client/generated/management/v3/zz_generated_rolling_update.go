package client

const (
	RollingUpdateType           = "rollingUpdate"
	RollingUpdateFieldBatchSize = "batchSize"
	RollingUpdateFieldInterval  = "interval"
)

type RollingUpdate struct {
	BatchSize int64 `json:"batchSize,omitempty" yaml:"batchSize,omitempty"`
	Interval  int64 `json:"interval,omitempty" yaml:"interval,omitempty"`
}
