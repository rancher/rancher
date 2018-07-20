package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterPipelineType                      = "clusterPipeline"
	ClusterPipelineFieldAnnotations          = "annotations"
	ClusterPipelineFieldClusterID            = "clusterId"
	ClusterPipelineFieldCreated              = "created"
	ClusterPipelineFieldCreatorID            = "creatorId"
	ClusterPipelineFieldDeploy               = "deploy"
	ClusterPipelineFieldGithubConfig         = "githubConfig"
	ClusterPipelineFieldLabels               = "labels"
	ClusterPipelineFieldName                 = "name"
	ClusterPipelineFieldNamespaceId          = "namespaceId"
	ClusterPipelineFieldOwnerReferences      = "ownerReferences"
	ClusterPipelineFieldRemoved              = "removed"
	ClusterPipelineFieldState                = "state"
	ClusterPipelineFieldStatus               = "status"
	ClusterPipelineFieldTransitioning        = "transitioning"
	ClusterPipelineFieldTransitioningMessage = "transitioningMessage"
	ClusterPipelineFieldUUID                 = "uuid"
)

type ClusterPipeline struct {
	types.Resource
	Annotations          map[string]string      `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterID            string                 `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Created              string                 `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string                 `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Deploy               bool                   `json:"deploy,omitempty" yaml:"deploy,omitempty"`
	GithubConfig         *GithubClusterConfig   `json:"githubConfig,omitempty" yaml:"githubConfig,omitempty"`
	Labels               map[string]string      `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string                 `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string                 `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference       `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed              string                 `json:"removed,omitempty" yaml:"removed,omitempty"`
	State                string                 `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *ClusterPipelineStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning        string                 `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string                 `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string                 `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type ClusterPipelineCollection struct {
	types.Collection
	Data   []ClusterPipeline `json:"data,omitempty"`
	client *ClusterPipelineClient
}

type ClusterPipelineClient struct {
	apiClient *Client
}

type ClusterPipelineOperations interface {
	List(opts *types.ListOpts) (*ClusterPipelineCollection, error)
	Create(opts *ClusterPipeline) (*ClusterPipeline, error)
	Update(existing *ClusterPipeline, updates interface{}) (*ClusterPipeline, error)
	Replace(existing *ClusterPipeline) (*ClusterPipeline, error)
	ByID(id string) (*ClusterPipeline, error)
	Delete(container *ClusterPipeline) error

	ActionAuthapp(resource *ClusterPipeline, input *AuthAppInput) (*ClusterPipeline, error)

	ActionAuthuser(resource *ClusterPipeline, input *AuthUserInput) (*SourceCodeCredential, error)

	ActionDeploy(resource *ClusterPipeline) error

	ActionDestroy(resource *ClusterPipeline) error

	ActionRevokeapp(resource *ClusterPipeline) error
}

func newClusterPipelineClient(apiClient *Client) *ClusterPipelineClient {
	return &ClusterPipelineClient{
		apiClient: apiClient,
	}
}

func (c *ClusterPipelineClient) Create(container *ClusterPipeline) (*ClusterPipeline, error) {
	resp := &ClusterPipeline{}
	err := c.apiClient.Ops.DoCreate(ClusterPipelineType, container, resp)
	return resp, err
}

func (c *ClusterPipelineClient) Update(existing *ClusterPipeline, updates interface{}) (*ClusterPipeline, error) {
	resp := &ClusterPipeline{}
	err := c.apiClient.Ops.DoUpdate(ClusterPipelineType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ClusterPipelineClient) Replace(obj *ClusterPipeline) (*ClusterPipeline, error) {
	resp := &ClusterPipeline{}
	err := c.apiClient.Ops.DoReplace(ClusterPipelineType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ClusterPipelineClient) List(opts *types.ListOpts) (*ClusterPipelineCollection, error) {
	resp := &ClusterPipelineCollection{}
	err := c.apiClient.Ops.DoList(ClusterPipelineType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ClusterPipelineCollection) Next() (*ClusterPipelineCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ClusterPipelineCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ClusterPipelineClient) ByID(id string) (*ClusterPipeline, error) {
	resp := &ClusterPipeline{}
	err := c.apiClient.Ops.DoByID(ClusterPipelineType, id, resp)
	return resp, err
}

func (c *ClusterPipelineClient) Delete(container *ClusterPipeline) error {
	return c.apiClient.Ops.DoResourceDelete(ClusterPipelineType, &container.Resource)
}

func (c *ClusterPipelineClient) ActionAuthapp(resource *ClusterPipeline, input *AuthAppInput) (*ClusterPipeline, error) {
	resp := &ClusterPipeline{}
	err := c.apiClient.Ops.DoAction(ClusterPipelineType, "authapp", &resource.Resource, input, resp)
	return resp, err
}

func (c *ClusterPipelineClient) ActionAuthuser(resource *ClusterPipeline, input *AuthUserInput) (*SourceCodeCredential, error) {
	resp := &SourceCodeCredential{}
	err := c.apiClient.Ops.DoAction(ClusterPipelineType, "authuser", &resource.Resource, input, resp)
	return resp, err
}

func (c *ClusterPipelineClient) ActionDeploy(resource *ClusterPipeline) error {
	err := c.apiClient.Ops.DoAction(ClusterPipelineType, "deploy", &resource.Resource, nil, nil)
	return err
}

func (c *ClusterPipelineClient) ActionDestroy(resource *ClusterPipeline) error {
	err := c.apiClient.Ops.DoAction(ClusterPipelineType, "destroy", &resource.Resource, nil, nil)
	return err
}

func (c *ClusterPipelineClient) ActionRevokeapp(resource *ClusterPipeline) error {
	err := c.apiClient.Ops.DoAction(ClusterPipelineType, "revokeapp", &resource.Resource, nil, nil)
	return err
}
