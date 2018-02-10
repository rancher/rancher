package client

const (
	RBDVolumeSourceType              = "rbdVolumeSource"
	RBDVolumeSourceFieldCephMonitors = "monitors"
	RBDVolumeSourceFieldFSType       = "fsType"
	RBDVolumeSourceFieldKeyring      = "keyring"
	RBDVolumeSourceFieldRBDImage     = "image"
	RBDVolumeSourceFieldRBDPool      = "pool"
	RBDVolumeSourceFieldRadosUser    = "user"
	RBDVolumeSourceFieldReadOnly     = "readOnly"
	RBDVolumeSourceFieldSecretRef    = "secretRef"
)

type RBDVolumeSource struct {
	CephMonitors []string              `json:"monitors,omitempty"`
	FSType       string                `json:"fsType,omitempty"`
	Keyring      string                `json:"keyring,omitempty"`
	RBDImage     string                `json:"image,omitempty"`
	RBDPool      string                `json:"pool,omitempty"`
	RadosUser    string                `json:"user,omitempty"`
	ReadOnly     *bool                 `json:"readOnly,omitempty"`
	SecretRef    *LocalObjectReference `json:"secretRef,omitempty"`
}
