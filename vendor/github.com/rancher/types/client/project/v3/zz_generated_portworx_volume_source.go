package client

const (
	PortworxVolumeSourceType          = "portworxVolumeSource"
	PortworxVolumeSourceFieldFSType   = "fsType"
	PortworxVolumeSourceFieldReadOnly = "readOnly"
	PortworxVolumeSourceFieldVolumeID = "volumeID"
)

type PortworxVolumeSource struct {
	FSType   string `json:"fsType,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty"`
	VolumeID string `json:"volumeID,omitempty"`
}
