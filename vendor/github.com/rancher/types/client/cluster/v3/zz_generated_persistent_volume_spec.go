package client

const (
	PersistentVolumeSpecType                               = "persistentVolumeSpec"
	PersistentVolumeSpecFieldAWSElasticBlockStore          = "awsElasticBlockStore"
	PersistentVolumeSpecFieldAccessModes                   = "accessModes"
	PersistentVolumeSpecFieldAzureDisk                     = "azureDisk"
	PersistentVolumeSpecFieldAzureFile                     = "azureFile"
	PersistentVolumeSpecFieldCapacity                      = "capacity"
	PersistentVolumeSpecFieldCephFS                        = "cephfs"
	PersistentVolumeSpecFieldCinder                        = "cinder"
	PersistentVolumeSpecFieldClaimRef                      = "claimRef"
	PersistentVolumeSpecFieldFC                            = "fc"
	PersistentVolumeSpecFieldFlexVolume                    = "flexVolume"
	PersistentVolumeSpecFieldFlocker                       = "flocker"
	PersistentVolumeSpecFieldGCEPersistentDisk             = "gcePersistentDisk"
	PersistentVolumeSpecFieldGlusterfs                     = "glusterfs"
	PersistentVolumeSpecFieldHostPath                      = "hostPath"
	PersistentVolumeSpecFieldISCSI                         = "iscsi"
	PersistentVolumeSpecFieldLocal                         = "local"
	PersistentVolumeSpecFieldMountOptions                  = "mountOptions"
	PersistentVolumeSpecFieldNFS                           = "nfs"
	PersistentVolumeSpecFieldPersistentVolumeReclaimPolicy = "persistentVolumeReclaimPolicy"
	PersistentVolumeSpecFieldPhotonPersistentDisk          = "photonPersistentDisk"
	PersistentVolumeSpecFieldPortworxVolume                = "portworxVolume"
	PersistentVolumeSpecFieldQuobyte                       = "quobyte"
	PersistentVolumeSpecFieldRBD                           = "rbd"
	PersistentVolumeSpecFieldScaleIO                       = "scaleIO"
	PersistentVolumeSpecFieldStorageClassName              = "storageClassName"
	PersistentVolumeSpecFieldStorageOS                     = "storageos"
	PersistentVolumeSpecFieldVsphereVolume                 = "vsphereVolume"
)

type PersistentVolumeSpec struct {
	AWSElasticBlockStore          *AWSElasticBlockStoreVolumeSource `json:"awsElasticBlockStore,omitempty" yaml:"awsElasticBlockStore,omitempty"`
	AccessModes                   []string                          `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
	AzureDisk                     *AzureDiskVolumeSource            `json:"azureDisk,omitempty" yaml:"azureDisk,omitempty"`
	AzureFile                     *AzureFilePersistentVolumeSource  `json:"azureFile,omitempty" yaml:"azureFile,omitempty"`
	Capacity                      map[string]string                 `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	CephFS                        *CephFSPersistentVolumeSource     `json:"cephfs,omitempty" yaml:"cephfs,omitempty"`
	Cinder                        *CinderVolumeSource               `json:"cinder,omitempty" yaml:"cinder,omitempty"`
	ClaimRef                      *ObjectReference                  `json:"claimRef,omitempty" yaml:"claimRef,omitempty"`
	FC                            *FCVolumeSource                   `json:"fc,omitempty" yaml:"fc,omitempty"`
	FlexVolume                    *FlexVolumeSource                 `json:"flexVolume,omitempty" yaml:"flexVolume,omitempty"`
	Flocker                       *FlockerVolumeSource              `json:"flocker,omitempty" yaml:"flocker,omitempty"`
	GCEPersistentDisk             *GCEPersistentDiskVolumeSource    `json:"gcePersistentDisk,omitempty" yaml:"gcePersistentDisk,omitempty"`
	Glusterfs                     *GlusterfsVolumeSource            `json:"glusterfs,omitempty" yaml:"glusterfs,omitempty"`
	HostPath                      *HostPathVolumeSource             `json:"hostPath,omitempty" yaml:"hostPath,omitempty"`
	ISCSI                         *ISCSIVolumeSource                `json:"iscsi,omitempty" yaml:"iscsi,omitempty"`
	Local                         *LocalVolumeSource                `json:"local,omitempty" yaml:"local,omitempty"`
	MountOptions                  []string                          `json:"mountOptions,omitempty" yaml:"mountOptions,omitempty"`
	NFS                           *NFSVolumeSource                  `json:"nfs,omitempty" yaml:"nfs,omitempty"`
	PersistentVolumeReclaimPolicy string                            `json:"persistentVolumeReclaimPolicy,omitempty" yaml:"persistentVolumeReclaimPolicy,omitempty"`
	PhotonPersistentDisk          *PhotonPersistentDiskVolumeSource `json:"photonPersistentDisk,omitempty" yaml:"photonPersistentDisk,omitempty"`
	PortworxVolume                *PortworxVolumeSource             `json:"portworxVolume,omitempty" yaml:"portworxVolume,omitempty"`
	Quobyte                       *QuobyteVolumeSource              `json:"quobyte,omitempty" yaml:"quobyte,omitempty"`
	RBD                           *RBDVolumeSource                  `json:"rbd,omitempty" yaml:"rbd,omitempty"`
	ScaleIO                       *ScaleIOVolumeSource              `json:"scaleIO,omitempty" yaml:"scaleIO,omitempty"`
	StorageClassName              string                            `json:"storageClassName,omitempty" yaml:"storageClassName,omitempty"`
	StorageOS                     *StorageOSPersistentVolumeSource  `json:"storageos,omitempty" yaml:"storageos,omitempty"`
	VsphereVolume                 *VsphereVirtualDiskVolumeSource   `json:"vsphereVolume,omitempty" yaml:"vsphereVolume,omitempty"`
}
