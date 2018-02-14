package client

import (
	"github.com/rancher/norman/types"
)

const (
	ClusterPipelineType                      = "clusterPipeline"
	ClusterPipelineFieldAnnotations          = "annotations"
	ClusterPipelineFieldClusterId            = "clusterId"
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
	ClusterPipelineFieldUuid                 = "uuid"
)

type ClusterPipeline struct {
	types.Resource
	Annotations          map[string]string      `json:"annotations,omitempty"`
	ClusterId            string                 `json:"clusterId,omitempty"`
	Created              string                 `json:"created,omitempty"`
	CreatorID            string                 `json:"creatorId,omitempty"`
	Deploy               bool                   `json:"deploy,omitempty"`
	GithubConfig         *GithubClusterConfig   `json:"githubConfig,omitempty"`
	Labels               map[string]string      `json:"labels,omitempty"`
	Name                 string                 `json:"name,omitempty"`
	NamespaceId          string                 `json:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference       `json:"ownerReferences,omitempty"`
	Removed              string                 `json:"removed,omitempty"`
	State                string                 `json:"state,omitempty"`
	Status               *ClusterPipelineStatus `json:"status,omitempty"`
	Transitioning        string                 `json:"transitioning,omitempty"`
	TransitioningMessage string                 `json:"transitioningMessage,omitempty"`
	Uuid                 string                 `json:"uuid,omitempty"`
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
	ByID(id string) (*ClusterPipeline, error)
	Delete(container *ClusterPipeline) error

	ActionAuthuser(*ClusterPipeline, *AuthUserInput) (*SourceCodeCredential, error)
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

func (c *ClusterPipelineClient) ActionAuthuser(resource *ClusterPipeline, input *AuthUserInput) (*SourceCodeCredential, error) {

	resp := &SourceCodeCredential{}

	err := c.apiClient.Ops.DoAction(ClusterPipelineType, "authuser", &resource.Resource, input, resp)

	return resp, err
}
