package client

const (
	DownwardAPIVolumeFileType                  = "downwardAPIVolumeFile"
	DownwardAPIVolumeFileFieldFieldRef         = "fieldRef"
	DownwardAPIVolumeFileFieldMode             = "mode"
	DownwardAPIVolumeFileFieldPath             = "path"
	DownwardAPIVolumeFileFieldResourceFieldRef = "resourceFieldRef"
	DownwardAPIVolumeFileFieldUser             = "user"
)

type DownwardAPIVolumeFile struct {
	FieldRef         *ObjectFieldSelector   `json:"fieldRef,omitempty" yaml:"fieldRef,omitempty"`
	Mode             *int64                 `json:"mode,omitempty" yaml:"mode,omitempty"`
	Path             string                 `json:"path,omitempty" yaml:"path,omitempty"`
	ResourceFieldRef *ResourceFieldSelector `json:"resourceFieldRef,omitempty" yaml:"resourceFieldRef,omitempty"`
	User             *int64                 `json:"user,omitempty" yaml:"user,omitempty"`
}
