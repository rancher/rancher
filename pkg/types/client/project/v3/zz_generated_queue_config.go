package client

const (
	QueueConfigType                   = "queueConfig"
	QueueConfigFieldBatchSendDeadline = "batchSendDeadline"
	QueueConfigFieldCapacity          = "capacity"
	QueueConfigFieldMaxBackoff        = "maxBackoff"
	QueueConfigFieldMaxRetries        = "maxRetries"
	QueueConfigFieldMaxSamplesPerSend = "maxSamplesPerSend"
	QueueConfigFieldMaxShards         = "maxShards"
	QueueConfigFieldMinBackoff        = "minBackoff"
	QueueConfigFieldMinShards         = "minShards"
)

type QueueConfig struct {
	BatchSendDeadline string `json:"batchSendDeadline,omitempty" yaml:"batchSendDeadline,omitempty"`
	Capacity          int64  `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	MaxBackoff        string `json:"maxBackoff,omitempty" yaml:"maxBackoff,omitempty"`
	MaxRetries        int64  `json:"maxRetries,omitempty" yaml:"maxRetries,omitempty"`
	MaxSamplesPerSend int64  `json:"maxSamplesPerSend,omitempty" yaml:"maxSamplesPerSend,omitempty"`
	MaxShards         int64  `json:"maxShards,omitempty" yaml:"maxShards,omitempty"`
	MinBackoff        string `json:"minBackoff,omitempty" yaml:"minBackoff,omitempty"`
	MinShards         int64  `json:"minShards,omitempty" yaml:"minShards,omitempty"`
}
