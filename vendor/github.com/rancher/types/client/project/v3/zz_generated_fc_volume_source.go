package client

const (
	FCVolumeSourceType            = "fcVolumeSource"
	FCVolumeSourceFieldFSType     = "fsType"
	FCVolumeSourceFieldLun        = "lun"
	FCVolumeSourceFieldReadOnly   = "readOnly"
	FCVolumeSourceFieldTargetWWNs = "targetWWNs"
	FCVolumeSourceFieldWWIDs      = "wwids"
)

type FCVolumeSource struct {
	FSType     string   `json:"fsType,omitempty"`
	Lun        *int64   `json:"lun,omitempty"`
	ReadOnly   bool     `json:"readOnly,omitempty"`
	TargetWWNs []string `json:"targetWWNs,omitempty"`
	WWIDs      []string `json:"wwids,omitempty"`
}
