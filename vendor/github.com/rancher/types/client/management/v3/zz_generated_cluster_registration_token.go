package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterRegistrationTokenType                      = "clusterRegistrationToken"
	ClusterRegistrationTokenField                     = "creatorId"
	ClusterRegistrationTokenFieldAnnotations          = "annotations"
	ClusterRegistrationTokenFieldCreated              = "created"
	ClusterRegistrationTokenFieldFinalizers           = "finalizers"
	ClusterRegistrationTokenFieldLabels               = "labels"
	ClusterRegistrationTokenFieldName                 = "name"
	ClusterRegistrationTokenFieldOwnerReferences      = "ownerReferences"
	ClusterRegistrationTokenFieldRemoved              = "removed"
	ClusterRegistrationTokenFieldState                = "state"
	ClusterRegistrationTokenFieldStatus               = "status"
	ClusterRegistrationTokenFieldTransitioning        = "transitioning"
	ClusterRegistrationTokenFieldTransitioningMessage = "transitioningMessage"
	ClusterRegistrationTokenFieldUuid                 = "uuid"
)

type ClusterRegistrationToken struct {
	types.Resource
	string               `json:"creatorId,omitempty"`
	Annotations          map[string]string               `json:"annotations,omitempty"`
	Created              string                          `json:"created,omitempty"`
	Finalizers           []string                        `json:"finalizers,omitempty"`
	Labels               map[string]string               `json:"labels,omitempty"`
	Name                 string                          `json:"name,omitempty"`
	OwnerReferences      []OwnerReference                `json:"ownerReferences,omitempty"`
	Removed              string                          `json:"removed,omitempty"`
	State                string                          `json:"state,omitempty"`
	Status               *ClusterRegistrationTokenStatus `json:"status,omitempty"`
	Transitioning        string                          `json:"transitioning,omitempty"`
	TransitioningMessage string                          `json:"transitioningMessage,omitempty"`
	Uuid                 string                          `json:"uuid,omitempty"`
}
type ClusterRegistrationTokenCollection struct {
	types.Collection
	Data   []ClusterRegistrationToken `json:"data,omitempty"`
	client *ClusterRegistrationTokenClient
}

type ClusterRegistrationTokenClient struct {
	apiClient *Client
}

type ClusterRegistrationTokenOperations interface {
	List(opts *types.ListOpts) (*ClusterRegistrationTokenCollection, error)
	Create(opts *ClusterRegistrationToken) (*ClusterRegistrationToken, error)
	Update(existing *ClusterRegistrationToken, updates interface{}) (*ClusterRegistrationToken, error)
	ByID(id string) (*ClusterRegistrationToken, error)
	Delete(container *ClusterRegistrationToken) error
}

func newClusterRegistrationTokenClient(apiClient *Client) *ClusterRegistrationTokenClient {
	return &ClusterRegistrationTokenClient{
		apiClient: apiClient,
	}
}

func (c *ClusterRegistrationTokenClient) Create(container *ClusterRegistrationToken) (*ClusterRegistrationToken, error) {
	resp := &ClusterRegistrationToken{}
	err := c.apiClient.Ops.DoCreate(ClusterRegistrationTokenType, container, resp)
	return resp, err
}

func (c *ClusterRegistrationTokenClient) Update(existing *ClusterRegistrationToken, updates interface{}) (*ClusterRegistrationToken, error) {
	resp := &ClusterRegistrationToken{}
	err := c.apiClient.Ops.DoUpdate(ClusterRegistrationTokenType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterRegistrationTokenClient) List(opts *types.ListOpts) (*ClusterRegistrationTokenCollection, error) {
	resp := &ClusterRegistrationTokenCollection{}
	err := c.apiClient.Ops.DoList(ClusterRegistrationTokenType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ClusterRegistrationTokenCollection) Next() (*ClusterRegistrationTokenCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterRegistrationTokenCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterRegistrationTokenClient) ByID(id string) (*ClusterRegistrationToken, error) {
	resp := &ClusterRegistrationToken{}
	err := c.apiClient.Ops.DoByID(ClusterRegistrationTokenType, id, resp)
	return resp, err
}

func (c *ClusterRegistrationTokenClient) Delete(container *ClusterRegistrationToken) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterRegistrationTokenType, &container.Resource)
}
