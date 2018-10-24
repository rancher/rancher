package client

const (
	CinderPersistentVolumeSourceType           = "cinderPersistentVolumeSource"
	CinderPersistentVolumeSourceFieldFSType    = "fsType"
	CinderPersistentVolumeSourceFieldReadOnly  = "readOnly"
	CinderPersistentVolumeSourceFieldSecretRef = "secretRef"
	CinderPersistentVolumeSourceFieldVolumeID  = "volumeID"
)

type CinderPersistentVolumeSource struct {
	FSType    string           `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	ReadOnly  bool             `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SecretRef *SecretReference `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
	VolumeID  string           `json:"volumeID,omitempty" yaml:"volumeID,omitempty"`
}
