package client

import (
	"github.com/rancher/norman/types"
)

const (
	EtcdBackupType                      = "etcdBackup"
	EtcdBackupFieldAnnotations          = "annotations"
	EtcdBackupFieldBackupConfig         = "backupConfig"
	EtcdBackupFieldClusterID            = "clusterId"
	EtcdBackupFieldCreated              = "created"
	EtcdBackupFieldCreatorID            = "creatorId"
	EtcdBackupFieldFilename             = "filename"
	EtcdBackupFieldLabels               = "labels"
	EtcdBackupFieldManual               = "manual"
	EtcdBackupFieldName                 = "name"
	EtcdBackupFieldNamespaceId          = "namespaceId"
	EtcdBackupFieldOwnerReferences      = "ownerReferences"
	EtcdBackupFieldRemoved              = "removed"
	EtcdBackupFieldState                = "state"
	EtcdBackupFieldStatus               = "status"
	EtcdBackupFieldTransitioning        = "transitioning"
	EtcdBackupFieldTransitioningMessage = "transitioningMessage"
	EtcdBackupFieldUUID                 = "uuid"
)

type EtcdBackup struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	BackupConfig         *BackupConfig     `json:"backupConfig,omitempty" yaml:"backupConfig,omitempty"`
	ClusterID            string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Created              string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Filename             string            `json:"filename,omitempty" yaml:"filename,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Manual               bool              `json:"manual,omitempty" yaml:"manual,omitempty"`
	Name                 string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed              string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string            `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *EtcdBackupStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type EtcdBackupCollection struct {
	types.Collection
	Data   []EtcdBackup `json:"data,omitempty"`
	client *EtcdBackupClient
}

type EtcdBackupClient struct {
	apiClient *Client
}

type EtcdBackupOperations interface {
	List(opts *types.ListOpts) (*EtcdBackupCollection, error)
	ListAll(opts *types.ListOpts) (*EtcdBackupCollection, error)
	Create(opts *EtcdBackup) (*EtcdBackup, error)
	Update(existing *EtcdBackup, updates interface{}) (*EtcdBackup, error)
	Replace(existing *EtcdBackup) (*EtcdBackup, error)
	ByID(id string) (*EtcdBackup, error)
	Delete(container *EtcdBackup) error
}

func newEtcdBackupClient(apiClient *Client) *EtcdBackupClient {
	return &EtcdBackupClient{
		apiClient: apiClient,
	}
}

func (c *EtcdBackupClient) Create(container *EtcdBackup) (*EtcdBackup, error) {
	resp := &EtcdBackup{}
	err := c.apiClient.Ops.DoCreate(EtcdBackupType, container, resp)
	return resp, err
}

func (c *EtcdBackupClient) Update(existing *EtcdBackup, updates interface{}) (*EtcdBackup, error) {
	resp := &EtcdBackup{}
	err := c.apiClient.Ops.DoUpdate(EtcdBackupType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *EtcdBackupClient) Replace(obj *EtcdBackup) (*EtcdBackup, error) {
	resp := &EtcdBackup{}
	err := c.apiClient.Ops.DoReplace(EtcdBackupType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *EtcdBackupClient) List(opts *types.ListOpts) (*EtcdBackupCollection, error) {
	resp := &EtcdBackupCollection{}
	err := c.apiClient.Ops.DoList(EtcdBackupType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *EtcdBackupClient) ListAll(opts *types.ListOpts) (*EtcdBackupCollection, error) {
	resp := &EtcdBackupCollection{}
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

func (cc *EtcdBackupCollection) Next() (*EtcdBackupCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &EtcdBackupCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *EtcdBackupClient) ByID(id string) (*EtcdBackup, error) {
	resp := &EtcdBackup{}
	err := c.apiClient.Ops.DoByID(EtcdBackupType, id, resp)
	return resp, err
}

func (c *EtcdBackupClient) Delete(container *EtcdBackup) error {
	return c.apiClient.Ops.DoResourceDelete(EtcdBackupType, &container.Resource)
}
