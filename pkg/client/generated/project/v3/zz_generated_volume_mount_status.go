package client

const (
	VolumeMountStatusType                   = "volumeMountStatus"
	VolumeMountStatusFieldMountPath         = "mountPath"
	VolumeMountStatusFieldName              = "name"
	VolumeMountStatusFieldReadOnly          = "readOnly"
	VolumeMountStatusFieldRecursiveReadOnly = "recursiveReadOnly"
)

type VolumeMountStatus struct {
	MountPath         string `json:"mountPath,omitempty" yaml:"mountPath,omitempty"`
	Name              string `json:"name,omitempty" yaml:"name,omitempty"`
	ReadOnly          bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	RecursiveReadOnly string `json:"recursiveReadOnly,omitempty" yaml:"recursiveReadOnly,omitempty"`
}
