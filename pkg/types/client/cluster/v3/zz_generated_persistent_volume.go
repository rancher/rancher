package client

import (
	"github.com/rancher/norman/types"
)

const (
	PersistentVolumeType                               = "persistentVolume"
	PersistentVolumeFieldAWSElasticBlockStore          = "awsElasticBlockStore"
	PersistentVolumeFieldAccessModes                   = "accessModes"
	PersistentVolumeFieldAnnotations                   = "annotations"
	PersistentVolumeFieldAzureDisk                     = "azureDisk"
	PersistentVolumeFieldAzureFile                     = "azureFile"
	PersistentVolumeFieldCSI                           = "csi"
	PersistentVolumeFieldCapacity                      = "capacity"
	PersistentVolumeFieldCephFS                        = "cephfs"
	PersistentVolumeFieldCinder                        = "cinder"
	PersistentVolumeFieldClaimRef                      = "claimRef"
	PersistentVolumeFieldCreated                       = "created"
	PersistentVolumeFieldCreatorID                     = "creatorId"
	PersistentVolumeFieldDescription                   = "description"
	PersistentVolumeFieldFC                            = "fc"
	PersistentVolumeFieldFlexVolume                    = "flexVolume"
	PersistentVolumeFieldFlocker                       = "flocker"
	PersistentVolumeFieldGCEPersistentDisk             = "gcePersistentDisk"
	PersistentVolumeFieldGlusterfs                     = "glusterfs"
	PersistentVolumeFieldHostPath                      = "hostPath"
	PersistentVolumeFieldISCSI                         = "iscsi"
	PersistentVolumeFieldLabels                        = "labels"
	PersistentVolumeFieldLocal                         = "local"
	PersistentVolumeFieldMountOptions                  = "mountOptions"
	PersistentVolumeFieldNFS                           = "nfs"
	PersistentVolumeFieldName                          = "name"
	PersistentVolumeFieldNodeAffinity                  = "nodeAffinity"
	PersistentVolumeFieldOwnerReferences               = "ownerReferences"
	PersistentVolumeFieldPersistentVolumeReclaimPolicy = "persistentVolumeReclaimPolicy"
	PersistentVolumeFieldPhotonPersistentDisk          = "photonPersistentDisk"
	PersistentVolumeFieldPortworxVolume                = "portworxVolume"
	PersistentVolumeFieldQuobyte                       = "quobyte"
	PersistentVolumeFieldRBD                           = "rbd"
	PersistentVolumeFieldRemoved                       = "removed"
	PersistentVolumeFieldScaleIO                       = "scaleIO"
	PersistentVolumeFieldState                         = "state"
	PersistentVolumeFieldStatus                        = "status"
	PersistentVolumeFieldStorageClassID                = "storageClassId"
	PersistentVolumeFieldStorageOS                     = "storageos"
	PersistentVolumeFieldTransitioning                 = "transitioning"
	PersistentVolumeFieldTransitioningMessage          = "transitioningMessage"
	PersistentVolumeFieldUUID                          = "uuid"
	PersistentVolumeFieldVolumeMode                    = "volumeMode"
	PersistentVolumeFieldVsphereVolume                 = "vsphereVolume"
)

type PersistentVolume struct {
	types.Resource
	AWSElasticBlockStore          *AWSElasticBlockStoreVolumeSource `json:"awsElasticBlockStore,omitempty" yaml:"awsElasticBlockStore,omitempty"`
	AccessModes                   []string                          `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
	Annotations                   map[string]string                 `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AzureDisk                     *AzureDiskVolumeSource            `json:"azureDisk,omitempty" yaml:"azureDisk,omitempty"`
	AzureFile                     *AzureFilePersistentVolumeSource  `json:"azureFile,omitempty" yaml:"azureFile,omitempty"`
	CSI                           *CSIPersistentVolumeSource        `json:"csi,omitempty" yaml:"csi,omitempty"`
	Capacity                      map[string]string                 `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	CephFS                        *CephFSPersistentVolumeSource     `json:"cephfs,omitempty" yaml:"cephfs,omitempty"`
	Cinder                        *CinderPersistentVolumeSource     `json:"cinder,omitempty" yaml:"cinder,omitempty"`
	ClaimRef                      *ObjectReference                  `json:"claimRef,omitempty" yaml:"claimRef,omitempty"`
	Created                       string                            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                     string                            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description                   string                            `json:"description,omitempty" yaml:"description,omitempty"`
	FC                            *FCVolumeSource                   `json:"fc,omitempty" yaml:"fc,omitempty"`
	FlexVolume                    *FlexPersistentVolumeSource       `json:"flexVolume,omitempty" yaml:"flexVolume,omitempty"`
	Flocker                       *FlockerVolumeSource              `json:"flocker,omitempty" yaml:"flocker,omitempty"`
	GCEPersistentDisk             *GCEPersistentDiskVolumeSource    `json:"gcePersistentDisk,omitempty" yaml:"gcePersistentDisk,omitempty"`
	Glusterfs                     *GlusterfsPersistentVolumeSource  `json:"glusterfs,omitempty" yaml:"glusterfs,omitempty"`
	HostPath                      *HostPathVolumeSource             `json:"hostPath,omitempty" yaml:"hostPath,omitempty"`
	ISCSI                         *ISCSIPersistentVolumeSource      `json:"iscsi,omitempty" yaml:"iscsi,omitempty"`
	Labels                        map[string]string                 `json:"labels,omitempty" yaml:"labels,omitempty"`
	Local                         *LocalVolumeSource                `json:"local,omitempty" yaml:"local,omitempty"`
	MountOptions                  []string                          `json:"mountOptions,omitempty" yaml:"mountOptions,omitempty"`
	NFS                           *NFSVolumeSource                  `json:"nfs,omitempty" yaml:"nfs,omitempty"`
	Name                          string                            `json:"name,omitempty" yaml:"name,omitempty"`
	NodeAffinity                  *VolumeNodeAffinity               `json:"nodeAffinity,omitempty" yaml:"nodeAffinity,omitempty"`
	OwnerReferences               []OwnerReference                  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PersistentVolumeReclaimPolicy string                            `json:"persistentVolumeReclaimPolicy,omitempty" yaml:"persistentVolumeReclaimPolicy,omitempty"`
	PhotonPersistentDisk          *PhotonPersistentDiskVolumeSource `json:"photonPersistentDisk,omitempty" yaml:"photonPersistentDisk,omitempty"`
	PortworxVolume                *PortworxVolumeSource             `json:"portworxVolume,omitempty" yaml:"portworxVolume,omitempty"`
	Quobyte                       *QuobyteVolumeSource              `json:"quobyte,omitempty" yaml:"quobyte,omitempty"`
	RBD                           *RBDPersistentVolumeSource        `json:"rbd,omitempty" yaml:"rbd,omitempty"`
	Removed                       string                            `json:"removed,omitempty" yaml:"removed,omitempty"`
	ScaleIO                       *ScaleIOPersistentVolumeSource    `json:"scaleIO,omitempty" yaml:"scaleIO,omitempty"`
	State                         string                            `json:"state,omitempty" yaml:"state,omitempty"`
	Status                        *PersistentVolumeStatus           `json:"status,omitempty" yaml:"status,omitempty"`
	StorageClassID                string                            `json:"storageClassId,omitempty" yaml:"storageClassId,omitempty"`
	StorageOS                     *StorageOSPersistentVolumeSource  `json:"storageos,omitempty" yaml:"storageos,omitempty"`
	Transitioning                 string                            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage          string                            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                          string                            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	VolumeMode                    string                            `json:"volumeMode,omitempty" yaml:"volumeMode,omitempty"`
	VsphereVolume                 *VsphereVirtualDiskVolumeSource   `json:"vsphereVolume,omitempty" yaml:"vsphereVolume,omitempty"`
}

type PersistentVolumeCollection struct {
	types.Collection
	Data   []PersistentVolume `json:"data,omitempty"`
	client *PersistentVolumeClient
}

type PersistentVolumeClient struct {
	apiClient *Client
}

type PersistentVolumeOperations interface {
	List(opts *types.ListOpts) (*PersistentVolumeCollection, error)
	ListAll(opts *types.ListOpts) (*PersistentVolumeCollection, error)
	Create(opts *PersistentVolume) (*PersistentVolume, error)
	Update(existing *PersistentVolume, updates interface{}) (*PersistentVolume, error)
	Replace(existing *PersistentVolume) (*PersistentVolume, error)
	ByID(id string) (*PersistentVolume, error)
	Delete(container *PersistentVolume) error
}

func newPersistentVolumeClient(apiClient *Client) *PersistentVolumeClient {
	return &PersistentVolumeClient{
		apiClient: apiClient,
	}
}

func (c *PersistentVolumeClient) Create(container *PersistentVolume) (*PersistentVolume, error) {
	resp := &PersistentVolume{}
	err := c.apiClient.Ops.DoCreate(PersistentVolumeType, container, resp)
	return resp, err
}

func (c *PersistentVolumeClient) Update(existing *PersistentVolume, updates interface{}) (*PersistentVolume, error) {
	resp := &PersistentVolume{}
	err := c.apiClient.Ops.DoUpdate(PersistentVolumeType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PersistentVolumeClient) Replace(obj *PersistentVolume) (*PersistentVolume, error) {
	resp := &PersistentVolume{}
	err := c.apiClient.Ops.DoReplace(PersistentVolumeType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *PersistentVolumeClient) List(opts *types.ListOpts) (*PersistentVolumeCollection, error) {
	resp := &PersistentVolumeCollection{}
	err := c.apiClient.Ops.DoList(PersistentVolumeType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *PersistentVolumeClient) ListAll(opts *types.ListOpts) (*PersistentVolumeCollection, error) {
	resp := &PersistentVolumeCollection{}
	resp, err := c.List(opts)
	if err != nil {
		return resp, err
	}
	data := resp.Data
	for next, err := resp.Next(); next != nil && err == nil; next, err = next.Next() {
		data = append(data, next.Data...)
		resp = next
		resp.Data = data
	}
	if err != nil {
		return resp, err
	}
	return resp, err
}

func (cc *PersistentVolumeCollection) Next() (*PersistentVolumeCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PersistentVolumeCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PersistentVolumeClient) ByID(id string) (*PersistentVolume, error) {
	resp := &PersistentVolume{}
	err := c.apiClient.Ops.DoByID(PersistentVolumeType, id, resp)
	return resp, err
}

func (c *PersistentVolumeClient) Delete(container *PersistentVolume) error {
	return c.apiClient.Ops.DoResourceDelete(PersistentVolumeType, &container.Resource)
}
