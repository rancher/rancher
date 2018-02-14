package client

const (
	CephFSVolumeSourceType            = "cephFSVolumeSource"
	CephFSVolumeSourceFieldMonitors   = "monitors"
	CephFSVolumeSourceFieldPath       = "path"
	CephFSVolumeSourceFieldReadOnly   = "readOnly"
	CephFSVolumeSourceFieldSecretFile = "secretFile"
	CephFSVolumeSourceFieldSecretRef  = "secretRef"
	CephFSVolumeSourceFieldUser       = "user"
)

type CephFSVolumeSource struct {
	Monitors   []string              `json:"monitors,omitempty"`
	Path       string                `json:"path,omitempty"`
	ReadOnly   bool                  `json:"readOnly,omitempty"`
	SecretFile string                `json:"secretFile,omitempty"`
	SecretRef  *LocalObjectReference `json:"secretRef,omitempty"`
	User       string                `json:"user,omitempty"`
}
