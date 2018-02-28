package client

const (
	ReplicationControllerConfigType                 = "replicationControllerConfig"
	ReplicationControllerConfigFieldMinReadySeconds = "minReadySeconds"
)

type ReplicationControllerConfig struct {
	MinReadySeconds *int64 `json:"minReadySeconds,omitempty" yaml:"minReadySeconds,omitempty"`
}
