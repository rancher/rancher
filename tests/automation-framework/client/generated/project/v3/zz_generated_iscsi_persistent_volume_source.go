package client

const (
	ISCSIPersistentVolumeSourceType                   = "iscsiPersistentVolumeSource"
	ISCSIPersistentVolumeSourceFieldDiscoveryCHAPAuth = "chapAuthDiscovery"
	ISCSIPersistentVolumeSourceFieldFSType            = "fsType"
	ISCSIPersistentVolumeSourceFieldIQN               = "iqn"
	ISCSIPersistentVolumeSourceFieldISCSIInterface    = "iscsiInterface"
	ISCSIPersistentVolumeSourceFieldInitiatorName     = "initiatorName"
	ISCSIPersistentVolumeSourceFieldLun               = "lun"
	ISCSIPersistentVolumeSourceFieldPortals           = "portals"
	ISCSIPersistentVolumeSourceFieldReadOnly          = "readOnly"
	ISCSIPersistentVolumeSourceFieldSecretRef         = "secretRef"
	ISCSIPersistentVolumeSourceFieldSessionCHAPAuth   = "chapAuthSession"
	ISCSIPersistentVolumeSourceFieldTargetPortal      = "targetPortal"
)

type ISCSIPersistentVolumeSource struct {
	DiscoveryCHAPAuth bool             `json:"chapAuthDiscovery,omitempty" yaml:"chapAuthDiscovery,omitempty"`
	FSType            string           `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	IQN               string           `json:"iqn,omitempty" yaml:"iqn,omitempty"`
	ISCSIInterface    string           `json:"iscsiInterface,omitempty" yaml:"iscsiInterface,omitempty"`
	InitiatorName     string           `json:"initiatorName,omitempty" yaml:"initiatorName,omitempty"`
	Lun               int64            `json:"lun,omitempty" yaml:"lun,omitempty"`
	Portals           []string         `json:"portals,omitempty" yaml:"portals,omitempty"`
	ReadOnly          bool             `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	SecretRef         *SecretReference `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
	SessionCHAPAuth   bool             `json:"chapAuthSession,omitempty" yaml:"chapAuthSession,omitempty"`
	TargetPortal      string           `json:"targetPortal,omitempty" yaml:"targetPortal,omitempty"`
}
