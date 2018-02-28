package client

const (
	VolumeType                       = "volume"
	VolumeFieldAWSElasticBlockStore  = "awsElasticBlockStore"
	VolumeFieldAzureDisk             = "azureDisk"
	VolumeFieldAzureFile             = "azureFile"
	VolumeFieldCephFS                = "cephfs"
	VolumeFieldCinder                = "cinder"
	VolumeFieldConfigMap             = "configMap"
	VolumeFieldDownwardAPI           = "downwardAPI"
	VolumeFieldEmptyDir              = "emptyDir"
	VolumeFieldFC                    = "fc"
	VolumeFieldFlexVolume            = "flexVolume"
	VolumeFieldFlocker               = "flocker"
	VolumeFieldGCEPersistentDisk     = "gcePersistentDisk"
	VolumeFieldGitRepo               = "gitRepo"
	VolumeFieldGlusterfs             = "glusterfs"
	VolumeFieldHostPath              = "hostPath"
	VolumeFieldISCSI                 = "iscsi"
	VolumeFieldNFS                   = "nfs"
	VolumeFieldName                  = "name"
	VolumeFieldPersistentVolumeClaim = "persistentVolumeClaim"
	VolumeFieldPhotonPersistentDisk  = "photonPersistentDisk"
	VolumeFieldPortworxVolume        = "portworxVolume"
	VolumeFieldProjected             = "projected"
	VolumeFieldQuobyte               = "quobyte"
	VolumeFieldRBD                   = "rbd"
	VolumeFieldScaleIO               = "scaleIO"
	VolumeFieldSecret                = "secret"
	VolumeFieldStorageOS             = "storageos"
	VolumeFieldVsphereVolume         = "vsphereVolume"
)

type Volume struct {
	AWSElasticBlockStore  *AWSElasticBlockStoreVolumeSource  `json:"awsElasticBlockStore,omitempty" yaml:"awsElasticBlockStore,omitempty"`
	AzureDisk             *AzureDiskVolumeSource             `json:"azureDisk,omitempty" yaml:"azureDisk,omitempty"`
	AzureFile             *AzureFileVolumeSource             `json:"azureFile,omitempty" yaml:"azureFile,omitempty"`
	CephFS                *CephFSVolumeSource                `json:"cephfs,omitempty" yaml:"cephfs,omitempty"`
	Cinder                *CinderVolumeSource                `json:"cinder,omitempty" yaml:"cinder,omitempty"`
	ConfigMap             *ConfigMapVolumeSource             `json:"configMap,omitempty" yaml:"configMap,omitempty"`
	DownwardAPI           *DownwardAPIVolumeSource           `json:"downwardAPI,omitempty" yaml:"downwardAPI,omitempty"`
	EmptyDir              *EmptyDirVolumeSource              `json:"emptyDir,omitempty" yaml:"emptyDir,omitempty"`
	FC                    *FCVolumeSource                    `json:"fc,omitempty" yaml:"fc,omitempty"`
	FlexVolume            *FlexVolumeSource                  `json:"flexVolume,omitempty" yaml:"flexVolume,omitempty"`
	Flocker               *FlockerVolumeSource               `json:"flocker,omitempty" yaml:"flocker,omitempty"`
	GCEPersistentDisk     *GCEPersistentDiskVolumeSource     `json:"gcePersistentDisk,omitempty" yaml:"gcePersistentDisk,omitempty"`
	GitRepo               *GitRepoVolumeSource               `json:"gitRepo,omitempty" yaml:"gitRepo,omitempty"`
	Glusterfs             *GlusterfsVolumeSource             `json:"glusterfs,omitempty" yaml:"glusterfs,omitempty"`
	HostPath              *HostPathVolumeSource              `json:"hostPath,omitempty" yaml:"hostPath,omitempty"`
	ISCSI                 *ISCSIVolumeSource                 `json:"iscsi,omitempty" yaml:"iscsi,omitempty"`
	NFS                   *NFSVolumeSource                   `json:"nfs,omitempty" yaml:"nfs,omitempty"`
	Name                  string                             `json:"name,omitempty" yaml:"name,omitempty"`
	PersistentVolumeClaim *PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty" yaml:"persistentVolumeClaim,omitempty"`
	PhotonPersistentDisk  *PhotonPersistentDiskVolumeSource  `json:"photonPersistentDisk,omitempty" yaml:"photonPersistentDisk,omitempty"`
	PortworxVolume        *PortworxVolumeSource              `json:"portworxVolume,omitempty" yaml:"portworxVolume,omitempty"`
	Projected             *ProjectedVolumeSource             `json:"projected,omitempty" yaml:"projected,omitempty"`
	Quobyte               *QuobyteVolumeSource               `json:"quobyte,omitempty" yaml:"quobyte,omitempty"`
	RBD                   *RBDVolumeSource                   `json:"rbd,omitempty" yaml:"rbd,omitempty"`
	ScaleIO               *ScaleIOVolumeSource               `json:"scaleIO,omitempty" yaml:"scaleIO,omitempty"`
	Secret                *SecretVolumeSource                `json:"secret,omitempty" yaml:"secret,omitempty"`
	StorageOS             *StorageOSVolumeSource             `json:"storageos,omitempty" yaml:"storageos,omitempty"`
	VsphereVolume         *VsphereVirtualDiskVolumeSource    `json:"vsphereVolume,omitempty" yaml:"vsphereVolume,omitempty"`
}
