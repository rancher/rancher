package client

const (
	CephFSPersistentVolumeSourceType            = "cephFSPersistentVolumeSource"
	CephFSPersistentVolumeSourceFieldMonitors   = "monitors"
	CephFSPersistentVolumeSourceFieldPath       = "path"
	CephFSPersistentVolumeSourceFieldReadOnly   = "readOnly"
	CephFSPersistentVolumeSourceFieldSecretFile = "secretFile"
	CephFSPersistentVolumeSourceFieldSecretRef  = "secretRef"
	CephFSPersistentVolumeSourceFieldUser       = "user"
)

type CephFSPersistentVolumeSource struct {
	Monitors   []string         `json:"monitors,omitempty"`
	Path       string           `json:"path,omitempty"`
	ReadOnly   *bool            `json:"readOnly,omitempty"`
	SecretFile string           `json:"secretFile,omitempty"`
	SecretRef  *SecretReference `json:"secretRef,omitempty"`
	User       string           `json:"user,omitempty"`
}
