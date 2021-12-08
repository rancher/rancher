package client

const (
	PersistentVolumeClaimVolumeSourceType                         = "persistentVolumeClaimVolumeSource"
	PersistentVolumeClaimVolumeSourceFieldPersistentVolumeClaimID = "persistentVolumeClaimId"
	PersistentVolumeClaimVolumeSourceFieldReadOnly                = "readOnly"
)

type PersistentVolumeClaimVolumeSource struct {
	PersistentVolumeClaimID string `json:"persistentVolumeClaimId,omitempty" yaml:"persistentVolumeClaimId,omitempty"`
	ReadOnly                bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
}
