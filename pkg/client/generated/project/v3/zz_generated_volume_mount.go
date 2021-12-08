package client

const (
	VolumeMountType                  = "volumeMount"
	VolumeMountFieldMountPath        = "mountPath"
	VolumeMountFieldMountPropagation = "mountPropagation"
	VolumeMountFieldName             = "name"
	VolumeMountFieldReadOnly         = "readOnly"
	VolumeMountFieldSubPath          = "subPath"
	VolumeMountFieldSubPathExpr      = "subPathExpr"
)

type VolumeMount struct {
	MountPath        string `json:"mountPath,omitempty" yaml:"mountPath,omitempty"`
	MountPropagation string `json:"mountPropagation,omitempty" yaml:"mountPropagation,omitempty"`
	Name             string `json:"name,omitempty" yaml:"name,omitempty"`
	ReadOnly         bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SubPath          string `json:"subPath,omitempty" yaml:"subPath,omitempty"`
	SubPathExpr      string `json:"subPathExpr,omitempty" yaml:"subPathExpr,omitempty"`
}
