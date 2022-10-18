package client

const (
	ETCDSnapshotCreateType            = "etcdSnapshotCreate"
	ETCDSnapshotCreateFieldGeneration = "generation"
)

type ETCDSnapshotCreate struct {
	Generation int64 `json:"generation,omitempty" yaml:"generation,omitempty"`
}
