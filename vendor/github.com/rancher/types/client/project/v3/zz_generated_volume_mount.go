package client

const (
	VolumeMountType                  = "volumeMount"
	VolumeMountFieldMountPath        = "mountPath"
	VolumeMountFieldMountPropagation = "mountPropagation"
	VolumeMountFieldName             = "name"
	VolumeMountFieldReadOnly         = "readOnly"
	VolumeMountFieldSubPath          = "subPath"
)

type VolumeMount struct {
	MountPath        string `json:"mountPath,omitempty"`
	MountPropagation string `json:"mountPropagation,omitempty"`
	Name             string `json:"name,omitempty"`
	ReadOnly         bool   `json:"readOnly,omitempty"`
	SubPath          string `json:"subPath,omitempty"`
}
