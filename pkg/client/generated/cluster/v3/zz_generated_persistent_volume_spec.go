package client

const (
	PersistentVolumeSpecType                               = "persistentVolumeSpec"
	PersistentVolumeSpecFieldAWSElasticBlockStore          = "awsElasticBlockStore"
	PersistentVolumeSpecFieldAccessModes                   = "accessModes"
	PersistentVolumeSpecFieldAzureDisk                     = "azureDisk"
	PersistentVolumeSpecFieldAzureFile                     = "azureFile"
	PersistentVolumeSpecFieldCSI                           = "csi"
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
	PersistentVolumeSpecFieldNodeAffinity                  = "nodeAffinity"
	PersistentVolumeSpecFieldPersistentVolumeReclaimPolicy = "persistentVolumeReclaimPolicy"
	PersistentVolumeSpecFieldPhotonPersistentDisk          = "photonPersistentDisk"
	PersistentVolumeSpecFieldPortworxVolume                = "portworxVolume"
	PersistentVolumeSpecFieldQuobyte                       = "quobyte"
	PersistentVolumeSpecFieldRBD                           = "rbd"
	PersistentVolumeSpecFieldScaleIO                       = "scaleIO"
	PersistentVolumeSpecFieldStorageClassID                = "storageClassId"
	PersistentVolumeSpecFieldStorageOS                     = "storageos"
	PersistentVolumeSpecFieldVolumeMode                    = "volumeMode"
	PersistentVolumeSpecFieldVsphereVolume                 = "vsphereVolume"
)

type PersistentVolumeSpec struct {
	AWSElasticBlockStore          *AWSElasticBlockStoreVolumeSource `json:"awsElasticBlockStore,omitempty" yaml:"awsElasticBlockStore,omitempty"`
	AccessModes                   []string                          `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
	AzureDisk                     *AzureDiskVolumeSource            `json:"azureDisk,omitempty" yaml:"azureDisk,omitempty"`
	AzureFile                     *AzureFilePersistentVolumeSource  `json:"azureFile,omitempty" yaml:"azureFile,omitempty"`
	CSI                           *CSIPersistentVolumeSource        `json:"csi,omitempty" yaml:"csi,omitempty"`
	Capacity                      map[string]string                 `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	CephFS                        *CephFSPersistentVolumeSource     `json:"cephfs,omitempty" yaml:"cephfs,omitempty"`
	Cinder                        *CinderPersistentVolumeSource     `json:"cinder,omitempty" yaml:"cinder,omitempty"`
	ClaimRef                      *ObjectReference                  `json:"claimRef,omitempty" yaml:"claimRef,omitempty"`
	FC                            *FCVolumeSource                   `json:"fc,omitempty" yaml:"fc,omitempty"`
	FlexVolume                    *FlexPersistentVolumeSource       `json:"flexVolume,omitempty" yaml:"flexVolume,omitempty"`
	Flocker                       *FlockerVolumeSource              `json:"flocker,omitempty" yaml:"flocker,omitempty"`
	GCEPersistentDisk             *GCEPersistentDiskVolumeSource    `json:"gcePersistentDisk,omitempty" yaml:"gcePersistentDisk,omitempty"`
	Glusterfs                     *GlusterfsPersistentVolumeSource  `json:"glusterfs,omitempty" yaml:"glusterfs,omitempty"`
	HostPath                      *HostPathVolumeSource             `json:"hostPath,omitempty" yaml:"hostPath,omitempty"`
	ISCSI                         *ISCSIPersistentVolumeSource      `json:"iscsi,omitempty" yaml:"iscsi,omitempty"`
	Local                         *LocalVolumeSource                `json:"local,omitempty" yaml:"local,omitempty"`
	MountOptions                  []string                          `json:"mountOptions,omitempty" yaml:"mountOptions,omitempty"`
	NFS                           *NFSVolumeSource                  `json:"nfs,omitempty" yaml:"nfs,omitempty"`
	NodeAffinity                  *VolumeNodeAffinity               `json:"nodeAffinity,omitempty" yaml:"nodeAffinity,omitempty"`
	PersistentVolumeReclaimPolicy string                            `json:"persistentVolumeReclaimPolicy,omitempty" yaml:"persistentVolumeReclaimPolicy,omitempty"`
	PhotonPersistentDisk          *PhotonPersistentDiskVolumeSource `json:"photonPersistentDisk,omitempty" yaml:"photonPersistentDisk,omitempty"`
	PortworxVolume                *PortworxVolumeSource             `json:"portworxVolume,omitempty" yaml:"portworxVolume,omitempty"`
	Quobyte                       *QuobyteVolumeSource              `json:"quobyte,omitempty" yaml:"quobyte,omitempty"`
	RBD                           *RBDPersistentVolumeSource        `json:"rbd,omitempty" yaml:"rbd,omitempty"`
	ScaleIO                       *ScaleIOPersistentVolumeSource    `json:"scaleIO,omitempty" yaml:"scaleIO,omitempty"`
	StorageClassID                string                            `json:"storageClassId,omitempty" yaml:"storageClassId,omitempty"`
	StorageOS                     *StorageOSPersistentVolumeSource  `json:"storageos,omitempty" yaml:"storageos,omitempty"`
	VolumeMode                    string                            `json:"volumeMode,omitempty" yaml:"volumeMode,omitempty"`
	VsphereVolume                 *VsphereVirtualDiskVolumeSource   `json:"vsphereVolume,omitempty" yaml:"vsphereVolume,omitempty"`
}
