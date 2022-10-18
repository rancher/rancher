package client

const (
	PersistentVolumeClaimVolumeSourceType           = "persistentVolumeClaimVolumeSource"
	PersistentVolumeClaimVolumeSourceFieldClaimName = "claimName"
	PersistentVolumeClaimVolumeSourceFieldReadOnly  = "readOnly"
)

type PersistentVolumeClaimVolumeSource struct {
	ClaimName string `json:"claimName,omitempty" yaml:"claimName,omitempty"`
	ReadOnly  bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
}
