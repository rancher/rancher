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
	SourceCodeRepositoryFieldDefaultBranch          = "defaultBranch"
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
	Annotations            map[string]string           `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterId              string                      `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Created                string                      `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID              string                      `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DefaultBranch          string                      `json:"defaultBranch,omitempty" yaml:"defaultBranch,omitempty"`
	Labels                 map[string]string           `json:"labels,omitempty" yaml:"labels,omitempty"`
	Language               string                      `json:"language,omitempty" yaml:"language,omitempty"`
	Name                   string                      `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences        []OwnerReference            `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Permissions            *RepoPerm                   `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	Removed                string                      `json:"removed,omitempty" yaml:"removed,omitempty"`
	SourceCodeCredentialId string                      `json:"sourceCodeCredentialId,omitempty" yaml:"sourceCodeCredentialId,omitempty"`
	SourceCodeType         string                      `json:"sourceCodeType,omitempty" yaml:"sourceCodeType,omitempty"`
	State                  string                      `json:"state,omitempty" yaml:"state,omitempty"`
	Status                 *SourceCodeRepositoryStatus `json:"status,omitempty" yaml:"status,omitempty"`
	Transitioning          string                      `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage   string                      `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	URL                    string                      `json:"url,omitempty" yaml:"url,omitempty"`
	UserId                 string                      `json:"userId,omitempty" yaml:"userId,omitempty"`
	Uuid                   string                      `json:"uuid,omitempty" yaml:"uuid,omitempty"`
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
