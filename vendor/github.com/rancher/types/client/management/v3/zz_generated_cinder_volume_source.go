package client

const (
	CinderVolumeSourceType           = "cinderVolumeSource"
	CinderVolumeSourceFieldFSType    = "fsType"
	CinderVolumeSourceFieldReadOnly  = "readOnly"
	CinderVolumeSourceFieldSecretRef = "secretRef"
	CinderVolumeSourceFieldVolumeID  = "volumeID"
)

type CinderVolumeSource struct {
	FSType    string                `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	ReadOnly  bool                  `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SecretRef *LocalObjectReference `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
	VolumeID  string                `json:"volumeID,omitempty" yaml:"volumeID,omitempty"`
}
