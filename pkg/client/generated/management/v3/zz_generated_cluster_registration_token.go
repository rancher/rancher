package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterRegistrationTokenType                            = "clusterRegistrationToken"
	ClusterRegistrationTokenFieldAnnotations                = "annotations"
	ClusterRegistrationTokenFieldClusterID                  = "clusterId"
	ClusterRegistrationTokenFieldCommand                    = "command"
	ClusterRegistrationTokenFieldCreated                    = "created"
	ClusterRegistrationTokenFieldCreatorID                  = "creatorId"
	ClusterRegistrationTokenFieldInsecureCommand            = "insecureCommand"
	ClusterRegistrationTokenFieldInsecureNodeCommand        = "insecureNodeCommand"
	ClusterRegistrationTokenFieldInsecureWindowsNodeCommand = "insecureWindowsNodeCommand"
	ClusterRegistrationTokenFieldLabels                     = "labels"
	ClusterRegistrationTokenFieldManifestURL                = "manifestUrl"
	ClusterRegistrationTokenFieldName                       = "name"
	ClusterRegistrationTokenFieldNamespaceId                = "namespaceId"
	ClusterRegistrationTokenFieldNodeCommand                = "nodeCommand"
	ClusterRegistrationTokenFieldOwnerReferences            = "ownerReferences"
	ClusterRegistrationTokenFieldRemoved                    = "removed"
	ClusterRegistrationTokenFieldState                      = "state"
	ClusterRegistrationTokenFieldToken                      = "token"
	ClusterRegistrationTokenFieldTransitioning              = "transitioning"
	ClusterRegistrationTokenFieldTransitioningMessage       = "transitioningMessage"
	ClusterRegistrationTokenFieldUUID                       = "uuid"
	ClusterRegistrationTokenFieldWindowsNodeCommand         = "windowsNodeCommand"
)

type ClusterRegistrationToken struct {
	types.Resource
	Annotations                map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterID                  string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Command                    string            `json:"command,omitempty" yaml:"command,omitempty"`
	Created                    string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                  string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	InsecureCommand            string            `json:"insecureCommand,omitempty" yaml:"insecureCommand,omitempty"`
	InsecureNodeCommand        string            `json:"insecureNodeCommand,omitempty" yaml:"insecureNodeCommand,omitempty"`
	InsecureWindowsNodeCommand string            `json:"insecureWindowsNodeCommand,omitempty" yaml:"insecureWindowsNodeCommand,omitempty"`
	Labels                     map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	ManifestURL                string            `json:"manifestUrl,omitempty" yaml:"manifestUrl,omitempty"`
	Name                       string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId                string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NodeCommand                string            `json:"nodeCommand,omitempty" yaml:"nodeCommand,omitempty"`
	OwnerReferences            []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed                    string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                      string            `json:"state,omitempty" yaml:"state,omitempty"`
	Token                      string            `json:"token,omitempty" yaml:"token,omitempty"`
	Transitioning              string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage       string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                       string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	WindowsNodeCommand         string            `json:"windowsNodeCommand,omitempty" yaml:"windowsNodeCommand,omitempty"`
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
	ListAll(opts *types.ListOpts) (*ClusterRegistrationTokenCollection, error)
	Create(opts *ClusterRegistrationToken) (*ClusterRegistrationToken, error)
	Update(existing *ClusterRegistrationToken, updates interface{}) (*ClusterRegistrationToken, error)
	Replace(existing *ClusterRegistrationToken) (*ClusterRegistrationToken, error)
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

func (c *ClusterRegistrationTokenClient) Replace(obj *ClusterRegistrationToken) (*ClusterRegistrationToken, error) {
	resp := &ClusterRegistrationToken{}
	err := c.apiClient.Ops.DoReplace(ClusterRegistrationTokenType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ClusterRegistrationTokenClient) List(opts *types.ListOpts) (*ClusterRegistrationTokenCollection, error) {
	resp := &ClusterRegistrationTokenCollection{}
	err := c.apiClient.Ops.DoList(ClusterRegistrationTokenType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ClusterRegistrationTokenClient) ListAll(opts *types.ListOpts) (*ClusterRegistrationTokenCollection, error) {
	resp := &ClusterRegistrationTokenCollection{}
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
