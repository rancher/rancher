package client

const (
	PersistentVolumeClaimVolumeSourceType                         = "persistentVolumeClaimVolumeSource"
	PersistentVolumeClaimVolumeSourceFieldPersistentVolumeClaimId = "persistentVolumeClaimId"
	PersistentVolumeClaimVolumeSourceFieldReadOnly                = "readOnly"
)

type PersistentVolumeClaimVolumeSource struct {
	PersistentVolumeClaimId string `json:"persistentVolumeClaimId,omitempty"`
	ReadOnly                bool   `json:"readOnly,omitempty"`
}
