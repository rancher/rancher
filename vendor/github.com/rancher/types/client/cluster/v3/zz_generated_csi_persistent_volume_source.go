package client

const (
	CSIPersistentVolumeSourceType              = "csiPersistentVolumeSource"
	CSIPersistentVolumeSourceFieldDriver       = "driver"
	CSIPersistentVolumeSourceFieldReadOnly     = "readOnly"
	CSIPersistentVolumeSourceFieldVolumeHandle = "volumeHandle"
)

type CSIPersistentVolumeSource struct {
	Driver       string `json:"driver,omitempty" yaml:"driver,omitempty"`
	ReadOnly     bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	VolumeHandle string `json:"volumeHandle,omitempty" yaml:"volumeHandle,omitempty"`
}
