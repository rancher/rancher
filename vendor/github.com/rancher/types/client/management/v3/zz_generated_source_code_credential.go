package client

import (
	"github.com/rancher/norman/types"
)

const (
	SourceCodeCredentialType                      = "sourceCodeCredential"
	SourceCodeCredentialFieldAccessToken          = "accessToken"
	SourceCodeCredentialFieldAnnotations          = "annotations"
	SourceCodeCredentialFieldAvatarURL            = "avatarUrl"
	SourceCodeCredentialFieldClusterId            = "clusterId"
	SourceCodeCredentialFieldCreated              = "created"
	SourceCodeCredentialFieldCreatorID            = "creatorId"
	SourceCodeCredentialFieldDisplayName          = "displayName"
	SourceCodeCredentialFieldHTMLURL              = "htmlUrl"
	SourceCodeCredentialFieldLabels               = "labels"
	SourceCodeCredentialFieldLoginName            = "loginName"
	SourceCodeCredentialFieldName                 = "name"
	SourceCodeCredentialFieldOwnerReferences      = "ownerReferences"
	SourceCodeCredentialFieldRemoved              = "removed"
	SourceCodeCredentialFieldSourceCodeType       = "sourceCodeType"
	SourceCodeCredentialFieldState                = "state"
	SourceCodeCredentialFieldStatus               = "status"
	SourceCodeCredentialFieldTransitioning        = "transitioning"
	SourceCodeCredentialFieldTransitioningMessage = "transitioningMessage"
	SourceCodeCredentialFieldUserId               = "userId"
	SourceCodeCredentialFieldUuid                 = "uuid"
)

type SourceCodeCredential struct {
	types.Resource
	AccessToken          string                      `json:"accessToken,omitempty" yaml:"accessToken,omitempty"`
	Annotations          map[string]string           `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AvatarURL            string                      `json:"avatarUrl,omitempty" yaml:"avatarUrl,omitempty"`
	ClusterId            string                      `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Created              string                      `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string                      `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DisplayName          string                      `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	HTMLURL              string                      `json:"htmlUrl,omitempty" yaml:"htmlUrl,omitempty"`
	Labels               map[string]string           `json:"labels,omitempty" yaml:"labels,omitempty"`
	LoginName            string                      `json:"loginName,omitempty" yaml:"loginName,omitempty"`
	Name                 string                      `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences      []OwnerReference            `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Removed              string                      `json:"removed,omitempty" yaml:"removed,omitempty"`
	SourceCodeType       string                      `json:"sourceCodeType,omitempty" yaml:"sourceCodeType,omitempty"`
	State                string                      `json:"state,omitempty" yaml:"state,omitempty"`
	Status               *SourceCodeCredentialStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning        string                      `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string                      `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UserId               string                      `json:"userId,omitempty" yaml:"userId,omitempty"`
	Uuid                 string                      `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}
type SourceCodeCredentialCollection struct {
	types.Collection
	Data   []SourceCodeCredential `json:"data,omitempty"`
	client *SourceCodeCredentialClient
}

type SourceCodeCredentialClient struct {
	apiClient *Client
}

type SourceCodeCredentialOperations interface {
	List(opts *types.ListOpts) (*SourceCodeCredentialCollection, error)
	Create(opts *SourceCodeCredential) (*SourceCodeCredential, error)
	Update(existing *SourceCodeCredential, updates interface{}) (*SourceCodeCredential, error)
	ByID(id string) (*SourceCodeCredential, error)
	Delete(container *SourceCodeCredential) error

	ActionRefreshrepos(resource *SourceCodeCredential) error
}

func newSourceCodeCredentialClient(apiClient *Client) *SourceCodeCredentialClient {
	return &SourceCodeCredentialClient{
		apiClient: apiClient,
	}
}

func (c *SourceCodeCredentialClient) Create(container *SourceCodeCredential) (*SourceCodeCredential, error) {
	resp := &SourceCodeCredential{}
	err := c.apiClient.Ops.DoCreate(SourceCodeCredentialType, container, resp)
	return resp, err
}

func (c *SourceCodeCredentialClient) Update(existing *SourceCodeCredential, updates interface{}) (*SourceCodeCredential, error) {
	resp := &SourceCodeCredential{}
	err := c.apiClient.Ops.DoUpdate(SourceCodeCredentialType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *SourceCodeCredentialClient) List(opts *types.ListOpts) (*SourceCodeCredentialCollection, error) {
	resp := &SourceCodeCredentialCollection{}
	err := c.apiClient.Ops.DoList(SourceCodeCredentialType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *SourceCodeCredentialCollection) Next() (*SourceCodeCredentialCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &SourceCodeCredentialCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *SourceCodeCredentialClient) ByID(id string) (*SourceCodeCredential, error) {
	resp := &SourceCodeCredential{}
	err := c.apiClient.Ops.DoByID(SourceCodeCredentialType, id, resp)
	return resp, err
}

func (c *SourceCodeCredentialClient) Delete(container *SourceCodeCredential) error {
	return c.apiClient.Ops.DoResourceDelete(SourceCodeCredentialType, &container.Resource)
}

func (c *SourceCodeCredentialClient) ActionRefreshrepos(resource *SourceCodeCredential) error {
	err := c.apiClient.Ops.DoAction(SourceCodeCredentialType, "refreshrepos", &resource.Resource, nil, nil)
	return err
}
