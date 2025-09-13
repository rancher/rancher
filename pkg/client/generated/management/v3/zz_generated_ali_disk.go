package client

const (
	AliDiskType                      = "aliDisk"
	AliDiskFieldAutoSnapshotPolicyID = "autoSnapshotPolicyId"
	AliDiskFieldCategory             = "category"
	AliDiskFieldEncrypted            = "encrypted"
	AliDiskFieldSize                 = "size"
)

type AliDisk struct {
	AutoSnapshotPolicyID string `json:"autoSnapshotPolicyId,omitempty" yaml:"autoSnapshotPolicyId,omitempty"`
	Category             string `json:"category,omitempty" yaml:"category,omitempty"`
	Encrypted            string `json:"encrypted,omitempty" yaml:"encrypted,omitempty"`
	Size                 int64  `json:"size,omitempty" yaml:"size,omitempty"`
}
