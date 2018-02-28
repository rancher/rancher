package client

const (
	CinderVolumeSourceType          = "cinderVolumeSource"
	CinderVolumeSourceFieldFSType   = "fsType"
	CinderVolumeSourceFieldReadOnly = "readOnly"
	CinderVolumeSourceFieldVolumeID = "volumeID"
)

type CinderVolumeSource struct {
	FSType   string `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	VolumeID string `json:"volumeID,omitempty" yaml:"volumeID,omitempty"`
}
