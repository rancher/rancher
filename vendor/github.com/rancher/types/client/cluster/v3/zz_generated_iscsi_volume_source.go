package client

const (
	ISCSIVolumeSourceType                   = "iscsiVolumeSource"
	ISCSIVolumeSourceFieldDiscoveryCHAPAuth = "chapAuthDiscovery"
	ISCSIVolumeSourceFieldFSType            = "fsType"
	ISCSIVolumeSourceFieldIQN               = "iqn"
	ISCSIVolumeSourceFieldISCSIInterface    = "iscsiInterface"
	ISCSIVolumeSourceFieldInitiatorName     = "initiatorName"
	ISCSIVolumeSourceFieldLun               = "lun"
	ISCSIVolumeSourceFieldPortals           = "portals"
	ISCSIVolumeSourceFieldReadOnly          = "readOnly"
	ISCSIVolumeSourceFieldSecretRef         = "secretRef"
	ISCSIVolumeSourceFieldSessionCHAPAuth   = "chapAuthSession"
	ISCSIVolumeSourceFieldTargetPortal      = "targetPortal"
)

type ISCSIVolumeSource struct {
	DiscoveryCHAPAuth bool                  `json:"chapAuthDiscovery,omitempty"`
	FSType            string                `json:"fsType,omitempty"`
	IQN               string                `json:"iqn,omitempty"`
	ISCSIInterface    string                `json:"iscsiInterface,omitempty"`
	InitiatorName     string                `json:"initiatorName,omitempty"`
	Lun               *int64                `json:"lun,omitempty"`
	Portals           []string              `json:"portals,omitempty"`
	ReadOnly          bool                  `json:"readOnly,omitempty"`
	SecretRef         *LocalObjectReference `json:"secretRef,omitempty"`
	SessionCHAPAuth   bool                  `json:"chapAuthSession,omitempty"`
	TargetPortal      string                `json:"targetPortal,omitempty"`
}
