package client

import (
	"github.com/rancher/norman/types"
)

const (
	SourceCodeRepositoryType                        = "sourceCodeRepository"
	SourceCodeRepositoryFieldAnnotations            = "annotations"
	SourceCodeRepositoryFieldClusterId              = "clusterId"
	SourceCodeRepositoryFieldCreated                = "created"
	SourceCodeRepositoryFieldCreatorID              = "creatorId"
	SourceCodeRepositoryFieldLabels                 = "labels"
	SourceCodeRepositoryFieldLanguage               = "language"
	SourceCodeRepositoryFieldName                   = "name"
	SourceCodeRepositoryFieldOwnerReferences        = "ownerReferences"
	SourceCodeRepositoryFieldPermissions            = "permissions"
	SourceCodeRepositoryFieldRemoved                = "removed"
	SourceCodeRepositoryFieldSourceCodeCredentialId = "sourceCodeCredentialId"
	SourceCodeRepositoryFieldSourceCodeType         = "sourceCodeType"
	SourceCodeRepositoryFieldState                  = "state"
	SourceCodeRepositoryFieldStatus                 = "status"
	SourceCodeRepositoryFieldTransitioning          = "transitioning"
	SourceCodeRepositoryFieldTransitioningMessage   = "transitioningMessage"
	SourceCodeRepositoryFieldURL                    = "url"
	SourceCodeRepositoryFieldUserId                 = "userId"
	SourceCodeRepositoryFieldUuid                   = "uuid"
)

type SourceCodeRepository struct {
	types.Resource
	Annotations            map[string]string           `json:"annotations,omitempty"`
	ClusterId              string                      `json:"clusterId,omitempty"`
	Created                string                      `json:"created,omitempty"`
	CreatorID              string                      `json:"creatorId,omitempty"`
	Labels                 map[string]string           `json:"labels,omitempty"`
	Language               string                      `json:"language,omitempty"`
	Name                   string                      `json:"name,omitempty"`
	OwnerReferences        []OwnerReference            `json:"ownerReferences,omitempty"`
	Permissions            *RepoPerm                   `json:"permissions,omitempty"`
	Removed                string                      `json:"removed,omitempty"`
	SourceCodeCredentialId string                      `json:"sourceCodeCredentialId,omitempty"`
	SourceCodeType         string                      `json:"sourceCodeType,omitempty"`
	State                  string                      `json:"state,omitempty"`
	Status                 *SourceCodeRepositoryStatus `json:"status,omitempty"`
	Transitioning          string                      `json:"transitioning,omitempty"`
	TransitioningMessage   string                      `json:"transitioningMessage,omitempty"`
	URL                    string                      `json:"url,omitempty"`
	UserId                 string                      `json:"userId,omitempty"`
	Uuid                   string                      `json:"uuid,omitempty"`
}
type SourceCodeRepositoryCollection struct {
	types.Collection
	Data   []SourceCodeRepository `json:"data,omitempty"`
	client *SourceCodeRepositoryClient
}

type SourceCodeRepositoryClient struct {
	apiClient *Client
}

type SourceCodeRepositoryOperations interface {
	List(opts *types.ListOpts) (*SourceCodeRepositoryCollection, error)
	Create(opts *SourceCodeRepository) (*SourceCodeRepository, error)
	Update(existing *SourceCodeRepository, updates interface{}) (*SourceCodeRepository, error)
	ByID(id string) (*SourceCodeRepository, error)
	Delete(container *SourceCodeRepository) error
}

func newSourceCodeRepositoryClient(apiClient *Client) *SourceCodeRepositoryClient {
	return &SourceCodeRepositoryClient{
		apiClient: apiClient,
	}
}

func (c *SourceCodeRepositoryClient) Create(container *SourceCodeRepository) (*SourceCodeRepository, error) {
	resp := &SourceCodeRepository{}
	err := c.apiClient.Ops.DoCreate(SourceCodeRepositoryType, container, resp)
	return resp, err
}

func (c *SourceCodeRepositoryClient) Update(existing *SourceCodeRepository, updates interface{}) (*SourceCodeRepository, error) {
	resp := &SourceCodeRepository{}
	err := c.apiClient.Ops.DoUpdate(SourceCodeRepositoryType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *SourceCodeRepositoryClient) List(opts *types.ListOpts) (*SourceCodeRepositoryCollection, error) {
	resp := &SourceCodeRepositoryCollection{}
	err := c.apiClient.Ops.DoList(SourceCodeRepositoryType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *SourceCodeRepositoryCollection) Next() (*SourceCodeRepositoryCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &SourceCodeRepositoryCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *SourceCodeRepositoryClient) ByID(id string) (*SourceCodeRepository, error) {
	resp := &SourceCodeRepository{}
	err := c.apiClient.Ops.DoByID(SourceCodeRepositoryType, id, resp)
	return resp, err
}

func (c *SourceCodeRepositoryClient) Delete(container *SourceCodeRepository) error {
	return c.apiClient.Ops.DoResourceDelete(SourceCodeRepositoryType, &container.Resource)
}
