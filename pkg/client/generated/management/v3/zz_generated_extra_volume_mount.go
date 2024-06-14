package client

const (
	ExtraVolumeMountType                   = "extraVolumeMount"
	ExtraVolumeMountFieldMountPath         = "mountPath"
	ExtraVolumeMountFieldMountPropagation  = "mountPropagation"
	ExtraVolumeMountFieldName              = "name"
	ExtraVolumeMountFieldReadOnly          = "readOnly"
	ExtraVolumeMountFieldRecursiveReadOnly = "recursiveReadOnly"
	ExtraVolumeMountFieldSubPath           = "subPath"
	ExtraVolumeMountFieldSubPathExpr       = "subPathExpr"
)

type ExtraVolumeMount struct {
	MountPath         string `json:"mountPath,omitempty" yaml:"mountPath,omitempty"`
	MountPropagation  string `json:"mountPropagation,omitempty" yaml:"mountPropagation,omitempty"`
	Name              string `json:"name,omitempty" yaml:"name,omitempty"`
	ReadOnly          bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	RecursiveReadOnly string `json:"recursiveReadOnly,omitempty" yaml:"recursiveReadOnly,omitempty"`
	SubPath           string `json:"subPath,omitempty" yaml:"subPath,omitempty"`
	SubPathExpr       string `json:"subPathExpr,omitempty" yaml:"subPathExpr,omitempty"`
}
