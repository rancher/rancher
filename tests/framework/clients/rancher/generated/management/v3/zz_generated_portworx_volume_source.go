package client

const (
	PortworxVolumeSourceType          = "portworxVolumeSource"
	PortworxVolumeSourceFieldFSType   = "fsType"
	PortworxVolumeSourceFieldReadOnly = "readOnly"
	PortworxVolumeSourceFieldVolumeID = "volumeID"
)

type PortworxVolumeSource struct {
	FSType   string `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	VolumeID string `json:"volumeID,omitempty" yaml:"volumeID,omitempty"`
}
