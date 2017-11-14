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
	AWSElasticBlockStore  *AWSElasticBlockStoreVolumeSource  `json:"awsElasticBlockStore,omitempty"`
	AzureDisk             *AzureDiskVolumeSource             `json:"azureDisk,omitempty"`
	AzureFile             *AzureFileVolumeSource             `json:"azureFile,omitempty"`
	CephFS                *CephFSVolumeSource                `json:"cephfs,omitempty"`
	Cinder                *CinderVolumeSource                `json:"cinder,omitempty"`
	ConfigMap             *ConfigMapVolumeSource             `json:"configMap,omitempty"`
	DownwardAPI           *DownwardAPIVolumeSource           `json:"downwardAPI,omitempty"`
	EmptyDir              *EmptyDirVolumeSource              `json:"emptyDir,omitempty"`
	FC                    *FCVolumeSource                    `json:"fc,omitempty"`
	FlexVolume            *FlexVolumeSource                  `json:"flexVolume,omitempty"`
	Flocker               *FlockerVolumeSource               `json:"flocker,omitempty"`
	GCEPersistentDisk     *GCEPersistentDiskVolumeSource     `json:"gcePersistentDisk,omitempty"`
	GitRepo               *GitRepoVolumeSource               `json:"gitRepo,omitempty"`
	Glusterfs             *GlusterfsVolumeSource             `json:"glusterfs,omitempty"`
	HostPath              *HostPathVolumeSource              `json:"hostPath,omitempty"`
	ISCSI                 *ISCSIVolumeSource                 `json:"iscsi,omitempty"`
	NFS                   *NFSVolumeSource                   `json:"nfs,omitempty"`
	PersistentVolumeClaim *PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
	PhotonPersistentDisk  *PhotonPersistentDiskVolumeSource  `json:"photonPersistentDisk,omitempty"`
	PortworxVolume        *PortworxVolumeSource              `json:"portworxVolume,omitempty"`
	Projected             *ProjectedVolumeSource             `json:"projected,omitempty"`
	Quobyte               *QuobyteVolumeSource               `json:"quobyte,omitempty"`
	RBD                   *RBDVolumeSource                   `json:"rbd,omitempty"`
	ScaleIO               *ScaleIOVolumeSource               `json:"scaleIO,omitempty"`
	Secret                *SecretVolumeSource                `json:"secret,omitempty"`
	StorageOS             *StorageOSVolumeSource             `json:"storageos,omitempty"`
	VsphereVolume         *VsphereVirtualDiskVolumeSource    `json:"vsphereVolume,omitempty"`
}
