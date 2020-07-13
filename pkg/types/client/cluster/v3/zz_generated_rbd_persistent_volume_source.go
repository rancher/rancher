package client

const (
	RBDPersistentVolumeSourceType              = "rbdPersistentVolumeSource"
	RBDPersistentVolumeSourceFieldCephMonitors = "monitors"
	RBDPersistentVolumeSourceFieldFSType       = "fsType"
	RBDPersistentVolumeSourceFieldKeyring      = "keyring"
	RBDPersistentVolumeSourceFieldRBDImage     = "image"
	RBDPersistentVolumeSourceFieldRBDPool      = "pool"
	RBDPersistentVolumeSourceFieldRadosUser    = "user"
	RBDPersistentVolumeSourceFieldReadOnly     = "readOnly"
	RBDPersistentVolumeSourceFieldSecretRef    = "secretRef"
)

type RBDPersistentVolumeSource struct {
	CephMonitors []string         `json:"monitors,omitempty" yaml:"monitors,omitempty"`
	FSType       string           `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	Keyring      string           `json:"keyring,omitempty" yaml:"keyring,omitempty"`
	RBDImage     string           `json:"image,omitempty" yaml:"image,omitempty"`
	RBDPool      string           `json:"pool,omitempty" yaml:"pool,omitempty"`
	RadosUser    string           `json:"user,omitempty" yaml:"user,omitempty"`
	ReadOnly     bool             `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SecretRef    *SecretReference `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
}
