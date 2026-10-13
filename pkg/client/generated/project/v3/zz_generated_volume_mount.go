package client

const (
	VolumeMountType                   = "volumeMount"
	VolumeMountFieldBindMountOptions  = "bindMountOptions"
	VolumeMountFieldMountPath         = "mountPath"
	VolumeMountFieldMountPropagation  = "mountPropagation"
	VolumeMountFieldName              = "name"
	VolumeMountFieldReadOnly          = "readOnly"
	VolumeMountFieldRecursiveReadOnly = "recursiveReadOnly"
	VolumeMountFieldSubPath           = "subPath"
	VolumeMountFieldSubPathExpr       = "subPathExpr"
)

type VolumeMount struct {
	BindMountOptions  []string `json:"bindMountOptions,omitempty" yaml:"bindMountOptions,omitempty"`
	MountPath         string   `json:"mountPath,omitempty" yaml:"mountPath,omitempty"`
	MountPropagation  string   `json:"mountPropagation,omitempty" yaml:"mountPropagation,omitempty"`
	Name              string   `json:"name,omitempty" yaml:"name,omitempty"`
	ReadOnly          bool     `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	RecursiveReadOnly string   `json:"recursiveReadOnly,omitempty" yaml:"recursiveReadOnly,omitempty"`
	SubPath           string   `json:"subPath,omitempty" yaml:"subPath,omitempty"`
	SubPathExpr       string   `json:"subPathExpr,omitempty" yaml:"subPathExpr,omitempty"`
}
