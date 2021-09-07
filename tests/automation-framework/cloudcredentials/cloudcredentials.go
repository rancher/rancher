package cloudcredentials

import (
	"fmt"

	"github.com/rancher/norman/types"
)

const (
	CloudCredentialType = "cloudCredential"
)

type CloudCredential struct {
	types.Resource
	Annotations                  map[string]string             `json:"annotations,omitempty"`
	Created                      string                        `json:"created,omitempty"`
	CreatorID                    string                        `json:"creatorId,omitempty"`
	Description                  string                        `json:"description,omitempty"`
	Labels                       map[string]string             `json:"labels,omitempty"`
	Name                         string                        `json:"name,omitempty"`
	Removed                      string                        `json:"removed,omitempty"`
	S3CredentialConfig           *S3CredentialConfig           `json:"s3credentialConfig,omitempty"`
	DigitalOceanCredentialConfig *DigitalOceanCredentialConfig `json:"digitaloceancredentialConfig,omitempty"`
	UUID                         string                        `json:"uuid,omitempty"`
}

type CloudCredentialCollection struct {
	types.Collection
	Data   []CloudCredential `json:"data,omitempty"`
	client *Client
}

type CloudCredentialOperations interface {
	List(opts *types.ListOpts) (*CloudCredentialCollection, error)
	ListAll(opts *types.ListOpts) (*CloudCredentialCollection, error)
	Create(opts *CloudCredential) (*CloudCredential, error)
	Update(existing *CloudCredential, updates interface{}) (*CloudCredential, error)
	Replace(existing *CloudCredential) (*CloudCredential, error)
	ByID(id string) (*CloudCredential, error)
	Delete(container *CloudCredential) error
}

// Takes everything
func NewCloudCredentialConfig(cloudCredentialName, description, namespace string, cloudCredentialProviderSpec interface{}) (*CloudCredential, error) {
	cloudCredential := &CloudCredential{
		Name:        cloudCredentialName,
		Description: description,
	}

	// cloudCredentialProviderSpec.Type
	switch spec := cloudCredentialProviderSpec.(type) {
	case DigitalOceanCredentialConfig:
		cloudCredential.DigitalOceanCredentialConfig = &spec
	default:
		return nil, fmt.Errorf("not a supported type")
	}

	return cloudCredential, nil
}

func (c *Client) Create(container *CloudCredential) (*CloudCredential, error) {
	resp := &CloudCredential{}
	err := c.apiClient.Ops.DoCreate(CloudCredentialType, container, resp)
	return resp, err
}

func (c *Client) Update(existing *CloudCredential, updates interface{}) (*CloudCredential, error) {
	resp := &CloudCredential{}
	err := c.apiClient.Ops.DoUpdate(CloudCredentialType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *Client) Replace(obj *CloudCredential) (*CloudCredential, error) {
	resp := &CloudCredential{}
	err := c.apiClient.Ops.DoReplace(CloudCredentialType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *Client) List(opts *types.ListOpts) (*CloudCredentialCollection, error) {
	resp := &CloudCredentialCollection{}
	err := c.apiClient.Ops.DoList(CloudCredentialType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *Client) ListAll(opts *types.ListOpts) (*CloudCredentialCollection, error) {
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

func (cc *CloudCredentialCollection) Next() (*CloudCredentialCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &CloudCredentialCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *Client) ByID(id string) (*CloudCredential, error) {
	resp := &CloudCredential{}
	err := c.apiClient.Ops.DoByID(CloudCredentialType, id, resp)
	return resp, err
}

func (c *Client) Delete(container *CloudCredential) error {
	return c.apiClient.Ops.DoResourceDelete(CloudCredentialType, &container.Resource)
}
