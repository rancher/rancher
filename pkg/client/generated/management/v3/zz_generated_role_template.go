package client

import (
	"github.com/rancher/norman/types"
)

const (
	RoleTemplateType                       = "roleTemplate"
	RoleTemplateFieldAdministrative        = "administrative"
	RoleTemplateFieldAnnotations           = "annotations"
	RoleTemplateFieldBuiltin               = "builtin"
	RoleTemplateFieldClusterCreatorDefault = "clusterCreatorDefault"
	RoleTemplateFieldContext               = "context"
	RoleTemplateFieldCreated               = "created"
	RoleTemplateFieldCreatorID             = "creatorId"
	RoleTemplateFieldDescription           = "description"
	RoleTemplateFieldExternal              = "external"
	RoleTemplateFieldExternalRules         = "externalRules"
	RoleTemplateFieldHidden                = "hidden"
	RoleTemplateFieldLabels                = "labels"
	RoleTemplateFieldLocked                = "locked"
	RoleTemplateFieldName                  = "name"
	RoleTemplateFieldOwnerReferences       = "ownerReferences"
	RoleTemplateFieldProjectCreatorDefault = "projectCreatorDefault"
	RoleTemplateFieldRemoved               = "removed"
	RoleTemplateFieldRoleTemplateIDs       = "roleTemplateIds"
	RoleTemplateFieldRules                 = "rules"
	RoleTemplateFieldUUID                  = "uuid"
)

type RoleTemplate struct {
	types.Resource
	Administrative        bool              `json:"administrative,omitempty" yaml:"administrative,omitempty"`
	Annotations           map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Builtin               bool              `json:"builtin,omitempty" yaml:"builtin,omitempty"`
	ClusterCreatorDefault bool              `json:"clusterCreatorDefault,omitempty" yaml:"clusterCreatorDefault,omitempty"`
	Context               string            `json:"context,omitempty" yaml:"context,omitempty"`
	Created               string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID             string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description           string            `json:"description,omitempty" yaml:"description,omitempty"`
	External              bool              `json:"external,omitempty" yaml:"external,omitempty"`
	ExternalRules         []PolicyRule      `json:"externalRules,omitempty" yaml:"externalRules,omitempty"`
	Hidden                bool              `json:"hidden,omitempty" yaml:"hidden,omitempty"`
	Labels                map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Locked                bool              `json:"locked,omitempty" yaml:"locked,omitempty"`
	Name                  string            `json:"name,omitempty" yaml:"name,omitempty"`
	OwnerReferences       []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectCreatorDefault bool              `json:"projectCreatorDefault,omitempty" yaml:"projectCreatorDefault,omitempty"`
	Removed               string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	RoleTemplateIDs       []string          `json:"roleTemplateIds,omitempty" yaml:"roleTemplateIds,omitempty"`
	Rules                 []PolicyRule      `json:"rules,omitempty" yaml:"rules,omitempty"`
	UUID                  string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
}

type RoleTemplateCollection struct {
	types.Collection
	Data   []RoleTemplate `json:"data,omitempty"`
	client *RoleTemplateClient
}

type RoleTemplateClient struct {
	apiClient *Client
}

type RoleTemplateOperations interface {
	List(opts *types.ListOpts) (*RoleTemplateCollection, error)
	ListAll(opts *types.ListOpts) (*RoleTemplateCollection, error)
	Create(opts *RoleTemplate) (*RoleTemplate, error)
	Update(existing *RoleTemplate, updates interface{}) (*RoleTemplate, error)
	Replace(existing *RoleTemplate) (*RoleTemplate, error)
	ByID(id string) (*RoleTemplate, error)
	Delete(container *RoleTemplate) error
}

func newRoleTemplateClient(apiClient *Client) *RoleTemplateClient {
	return &RoleTemplateClient{
		apiClient: apiClient,
	}
}

func (c *RoleTemplateClient) Create(container *RoleTemplate) (*RoleTemplate, error) {
	resp := &RoleTemplate{}
	err := c.apiClient.Ops.DoCreate(RoleTemplateType, container, resp)
	return resp, err
}

func (c *RoleTemplateClient) Update(existing *RoleTemplate, updates interface{}) (*RoleTemplate, error) {
	resp := &RoleTemplate{}
	err := c.apiClient.Ops.DoUpdate(RoleTemplateType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *RoleTemplateClient) Replace(obj *RoleTemplate) (*RoleTemplate, error) {
	resp := &RoleTemplate{}
	err := c.apiClient.Ops.DoReplace(RoleTemplateType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *RoleTemplateClient) List(opts *types.ListOpts) (*RoleTemplateCollection, error) {
	resp := &RoleTemplateCollection{}
	err := c.apiClient.Ops.DoList(RoleTemplateType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *RoleTemplateClient) ListAll(opts *types.ListOpts) (*RoleTemplateCollection, error) {
	resp := &RoleTemplateCollection{}
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

func (cc *RoleTemplateCollection) Next() (*RoleTemplateCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &RoleTemplateCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *RoleTemplateClient) ByID(id string) (*RoleTemplate, error) {
	resp := &RoleTemplate{}
	err := c.apiClient.Ops.DoByID(RoleTemplateType, id, resp)
	return resp, err
}

func (c *RoleTemplateClient) Delete(container *RoleTemplate) error {
	return c.apiClient.Ops.DoResourceDelete(RoleTemplateType, &container.Resource)
}
