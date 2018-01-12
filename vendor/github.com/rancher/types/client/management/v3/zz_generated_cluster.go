package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterType                                     = "cluster"
	ClusterFieldAPIEndpoint                         = "apiEndpoint"
	ClusterFieldAllocatable                         = "allocatable"
	ClusterFieldAnnotations                         = "annotations"
	ClusterFieldAzureKubernetesServiceConfig        = "azureKubernetesServiceConfig"
	ClusterFieldCACert                              = "caCert"
	ClusterFieldCapacity                            = "capacity"
	ClusterFieldComponentStatuses                   = "componentStatuses"
	ClusterFieldConditions                          = "conditions"
	ClusterFieldCreated                             = "created"
	ClusterFieldCreatorID                           = "creatorId"
	ClusterFieldDefaultClusterRoleForProjectMembers = "defaultClusterRoleForProjectMembers"
	ClusterFieldDefaultPodSecurityPolicyTemplateId  = "defaultPodSecurityPolicyTemplateId"
	ClusterFieldDescription                         = "description"
	ClusterFieldDriver                              = "driver"
	ClusterFieldGoogleKubernetesEngineConfig        = "googleKubernetesEngineConfig"
	ClusterFieldId                                  = "id"
	ClusterFieldInternal                            = "internal"
	ClusterFieldLabels                              = "labels"
	ClusterFieldLimits                              = "limits"
	ClusterFieldName                                = "name"
	ClusterFieldNodes                               = "nodes"
	ClusterFieldOwnerReferences                     = "ownerReferences"
	ClusterFieldRancherKubernetesEngineConfig       = "rancherKubernetesEngineConfig"
	ClusterFieldRemoved                             = "removed"
	ClusterFieldRequested                           = "requested"
	ClusterFieldServiceAccountToken                 = "serviceAccountToken"
	ClusterFieldState                               = "state"
	ClusterFieldTransitioning                       = "transitioning"
	ClusterFieldTransitioningMessage                = "transitioningMessage"
	ClusterFieldUuid                                = "uuid"
)

type Cluster struct {
	types.Resource
	APIEndpoint                         string                         `json:"apiEndpoint,omitempty"`
	Allocatable                         map[string]string              `json:"allocatable,omitempty"`
	Annotations                         map[string]string              `json:"annotations,omitempty"`
	AzureKubernetesServiceConfig        *AzureKubernetesServiceConfig  `json:"azureKubernetesServiceConfig,omitempty"`
	CACert                              string                         `json:"caCert,omitempty"`
	Capacity                            map[string]string              `json:"capacity,omitempty"`
	ComponentStatuses                   []ClusterComponentStatus       `json:"componentStatuses,omitempty"`
	Conditions                          []ClusterCondition             `json:"conditions,omitempty"`
	Created                             string                         `json:"created,omitempty"`
	CreatorID                           string                         `json:"creatorId,omitempty"`
	DefaultClusterRoleForProjectMembers string                         `json:"defaultClusterRoleForProjectMembers,omitempty"`
	DefaultPodSecurityPolicyTemplateId  string                         `json:"defaultPodSecurityPolicyTemplateId,omitempty"`
	Description                         string                         `json:"description,omitempty"`
	Driver                              string                         `json:"driver,omitempty"`
	GoogleKubernetesEngineConfig        *GoogleKubernetesEngineConfig  `json:"googleKubernetesEngineConfig,omitempty"`
	Id                                  string                         `json:"id,omitempty"`
	Internal                            *bool                          `json:"internal,omitempty"`
	Labels                              map[string]string              `json:"labels,omitempty"`
	Limits                              map[string]string              `json:"limits,omitempty"`
	Name                                string                         `json:"name,omitempty"`
	Nodes                               []MachineConfig                `json:"nodes,omitempty"`
	OwnerReferences                     []OwnerReference               `json:"ownerReferences,omitempty"`
	RancherKubernetesEngineConfig       *RancherKubernetesEngineConfig `json:"rancherKubernetesEngineConfig,omitempty"`
	Removed                             string                         `json:"removed,omitempty"`
	Requested                           map[string]string              `json:"requested,omitempty"`
	ServiceAccountToken                 string                         `json:"serviceAccountToken,omitempty"`
	State                               string                         `json:"state,omitempty"`
	Transitioning                       string                         `json:"transitioning,omitempty"`
	TransitioningMessage                string                         `json:"transitioningMessage,omitempty"`
	Uuid                                string                         `json:"uuid,omitempty"`
}
type ClusterCollection struct {
	types.Collection
	Data   []Cluster `json:"data,omitempty"`
	client *ClusterClient
}

type ClusterClient struct {
	apiClient *Client
}

type ClusterOperations interface {
	List(opts *types.ListOpts) (*ClusterCollection, error)
	Create(opts *Cluster) (*Cluster, error)
	Update(existing *Cluster, updates interface{}) (*Cluster, error)
	ByID(id string) (*Cluster, error)
	Delete(container *Cluster) error
}

func newClusterClient(apiClient *Client) *ClusterClient {
	return &ClusterClient{
		apiClient: apiClient,
	}
}

func (c *ClusterClient) Create(container *Cluster) (*Cluster, error) {
	resp := &Cluster{}
	err := c.apiClient.Ops.DoCreate(ClusterType, container, resp)
	return resp, err
}

func (c *ClusterClient) Update(existing *Cluster, updates interface{}) (*Cluster, error) {
	resp := &Cluster{}
	err := c.apiClient.Ops.DoUpdate(ClusterType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterClient) List(opts *types.ListOpts) (*ClusterCollection, error) {
	resp := &ClusterCollection{}
	err := c.apiClient.Ops.DoList(ClusterType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ClusterCollection) Next() (*ClusterCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterClient) ByID(id string) (*Cluster, error) {
	resp := &Cluster{}
	err := c.apiClient.Ops.DoByID(ClusterType, id, resp)
	return resp, err
}

func (c *ClusterClient) Delete(container *Cluster) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterType, &container.Resource)
}
