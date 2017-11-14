package client

const (
	PersistentVolumeClaimVolumeSourceType           = "persistentVolumeClaimVolumeSource"
	PersistentVolumeClaimVolumeSourceFieldClaimName = "claimName"
	PersistentVolumeClaimVolumeSourceFieldReadOnly  = "readOnly"
)

type PersistentVolumeClaimVolumeSource struct {
	ClaimName string `json:"claimName,omitempty"`
	ReadOnly  *bool  `json:"readOnly,omitempty"`
}
