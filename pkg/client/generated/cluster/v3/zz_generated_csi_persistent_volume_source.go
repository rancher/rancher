package client

const (
	CSIPersistentVolumeSourceType                            = "csiPersistentVolumeSource"
	CSIPersistentVolumeSourceFieldControllerExpandSecretRef  = "controllerExpandSecretRef"
	CSIPersistentVolumeSourceFieldControllerPublishSecretRef = "controllerPublishSecretRef"
	CSIPersistentVolumeSourceFieldDriver                     = "driver"
	CSIPersistentVolumeSourceFieldFSType                     = "fsType"
	CSIPersistentVolumeSourceFieldNodeExpandSecretRef        = "nodeExpandSecretRef"
	CSIPersistentVolumeSourceFieldNodePublishSecretRef       = "nodePublishSecretRef"
	CSIPersistentVolumeSourceFieldNodeStageSecretRef         = "nodeStageSecretRef"
	CSIPersistentVolumeSourceFieldReadOnly                   = "readOnly"
	CSIPersistentVolumeSourceFieldVolumeAttributes           = "volumeAttributes"
	CSIPersistentVolumeSourceFieldVolumeHandle               = "volumeHandle"
)

type CSIPersistentVolumeSource struct {
	ControllerExpandSecretRef  *SecretReference  `json:"controllerExpandSecretRef,omitempty" yaml:"controllerExpandSecretRef,omitempty"`
	ControllerPublishSecretRef *SecretReference  `json:"controllerPublishSecretRef,omitempty" yaml:"controllerPublishSecretRef,omitempty"`
	Driver                     string            `json:"driver,omitempty" yaml:"driver,omitempty"`
	FSType                     string            `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	NodeExpandSecretRef        *SecretReference  `json:"nodeExpandSecretRef,omitempty" yaml:"nodeExpandSecretRef,omitempty"`
	NodePublishSecretRef       *SecretReference  `json:"nodePublishSecretRef,omitempty" yaml:"nodePublishSecretRef,omitempty"`
	NodeStageSecretRef         *SecretReference  `json:"nodeStageSecretRef,omitempty" yaml:"nodeStageSecretRef,omitempty"`
	ReadOnly                   bool              `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	VolumeAttributes           map[string]string `json:"volumeAttributes,omitempty" yaml:"volumeAttributes,omitempty"`
	VolumeHandle               string            `json:"volumeHandle,omitempty" yaml:"volumeHandle,omitempty"`
}
