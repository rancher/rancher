package client

const (
	ModifyVolumeStatusType                                 = "modifyVolumeStatus"
	ModifyVolumeStatusFieldStatus                          = "status"
	ModifyVolumeStatusFieldTargetVolumeAttributesClassName = "targetVolumeAttributesClassName"
)

type ModifyVolumeStatus struct {
	Status                          string `json:"status,omitempty" yaml:"status,omitempty"`
	TargetVolumeAttributesClassName string `json:"targetVolumeAttributesClassName,omitempty" yaml:"targetVolumeAttributesClassName,omitempty"`
}
