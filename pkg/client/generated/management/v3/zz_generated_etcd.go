package client

const (
	ETCDType                      = "etcd"
	ETCDFieldDisableSnapshots     = "disableSnapshots"
	ETCDFieldS3                   = "s3"
	ETCDFieldSnapshotRetention    = "snapshotRetention"
	ETCDFieldSnapshotScheduleCron = "snapshotScheduleCron"
)

type ETCD struct {
	DisableSnapshots     bool            `json:"disableSnapshots,omitempty" yaml:"disableSnapshots,omitempty"`
	S3                   *ETCDSnapshotS3 `json:"s3,omitempty" yaml:"s3,omitempty"`
	SnapshotRetention    int64           `json:"snapshotRetention,omitempty" yaml:"snapshotRetention,omitempty"`
	SnapshotScheduleCron string          `json:"snapshotScheduleCron,omitempty" yaml:"snapshotScheduleCron,omitempty"`
}
