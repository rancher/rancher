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
	AWSElasticBlockStore          *AWSElasticBlockStoreVolumeSource `json:"awsElasticBlockStore,omitempty"`
	AccessModes                   []string                          `json:"accessModes,omitempty"`
	AzureDisk                     *AzureDiskVolumeSource            `json:"azureDisk,omitempty"`
	AzureFile                     *AzureFilePersistentVolumeSource  `json:"azureFile,omitempty"`
	Capacity                      map[string]string                 `json:"capacity,omitempty"`
	CephFS                        *CephFSPersistentVolumeSource     `json:"cephfs,omitempty"`
	Cinder                        *CinderVolumeSource               `json:"cinder,omitempty"`
	ClaimRef                      *ObjectReference                  `json:"claimRef,omitempty"`
	FC                            *FCVolumeSource                   `json:"fc,omitempty"`
	FlexVolume                    *FlexVolumeSource                 `json:"flexVolume,omitempty"`
	Flocker                       *FlockerVolumeSource              `json:"flocker,omitempty"`
	GCEPersistentDisk             *GCEPersistentDiskVolumeSource    `json:"gcePersistentDisk,omitempty"`
	Glusterfs                     *GlusterfsVolumeSource            `json:"glusterfs,omitempty"`
	HostPath                      *HostPathVolumeSource             `json:"hostPath,omitempty"`
	ISCSI                         *ISCSIVolumeSource                `json:"iscsi,omitempty"`
	Local                         *LocalVolumeSource                `json:"local,omitempty"`
	MountOptions                  []string                          `json:"mountOptions,omitempty"`
	NFS                           *NFSVolumeSource                  `json:"nfs,omitempty"`
	PersistentVolumeReclaimPolicy string                            `json:"persistentVolumeReclaimPolicy,omitempty"`
	PhotonPersistentDisk          *PhotonPersistentDiskVolumeSource `json:"photonPersistentDisk,omitempty"`
	PortworxVolume                *PortworxVolumeSource             `json:"portworxVolume,omitempty"`
	Quobyte                       *QuobyteVolumeSource              `json:"quobyte,omitempty"`
	RBD                           *RBDVolumeSource                  `json:"rbd,omitempty"`
	ScaleIO                       *ScaleIOVolumeSource              `json:"scaleIO,omitempty"`
	StorageClassName              string                            `json:"storageClassName,omitempty"`
	StorageOS                     *StorageOSPersistentVolumeSource  `json:"storageos,omitempty"`
	VsphereVolume                 *VsphereVirtualDiskVolumeSource   `json:"vsphereVolume,omitempty"`
}
