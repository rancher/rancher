package client

import (
	"github.com/rancher/norman/types"
)

const (
	ListenConfigType                         = "listenConfig"
	ListenConfigFieldAlgorithm               = "algorithm"
	ListenConfigFieldAnnotations             = "annotations"
	ListenConfigFieldCACerts                 = "caCerts"
	ListenConfigFieldCN                      = "cn"
	ListenConfigFieldCert                    = "cert"
	ListenConfigFieldCertFingerprint         = "certFingerprint"
	ListenConfigFieldCreated                 = "created"
	ListenConfigFieldCreatorID               = "creatorId"
	ListenConfigFieldDescription             = "description"
	ListenConfigFieldDomains                 = "domains"
	ListenConfigFieldEnabled                 = "enabled"
	ListenConfigFieldExpiresAt               = "expiresAt"
	ListenConfigFieldGeneratedCerts          = "generatedCerts"
	ListenConfigFieldIssuedAt                = "issuedAt"
	ListenConfigFieldIssuer                  = "issuer"
	ListenConfigFieldKey                     = "key"
	ListenConfigFieldKeySize                 = "keySize"
	ListenConfigFieldKnownIPs                = "knownIps"
	ListenConfigFieldLabels                  = "labels"
	ListenConfigFieldMode                    = "mode"
	ListenConfigFieldName                    = "name"
	ListenConfigFieldOwnerReferences         = "ownerReferences"
	ListenConfigFieldRemoved                 = "removed"
	ListenConfigFieldSerialNumber            = "serialNumber"
	ListenConfigFieldSubjectAlternativeNames = "subjectAlternativeNames"
	ListenConfigFieldTOS                     = "tos"
	ListenConfigFieldUuid                    = "uuid"
	ListenConfigFieldVersion                 = "version"
)

type ListenConfig struct {
	types.Resource
	Algorithm               string            `json:"algorithm,omitempty"`
	Annotations             map[string]string `json:"annotations,omitempty"`
	CACerts                 string            `json:"caCerts,omitempty"`
	CN                      string            `json:"cn,omitempty"`
	Cert                    string            `json:"cert,omitempty"`
	CertFingerprint         string            `json:"certFingerprint,omitempty"`
	Created                 string            `json:"created,omitempty"`
	CreatorID               string            `json:"creatorId,omitempty"`
	Description             string            `json:"description,omitempty"`
	Domains                 []string          `json:"domains,omitempty"`
	Enabled                 *bool             `json:"enabled,omitempty"`
	ExpiresAt               string            `json:"expiresAt,omitempty"`
	GeneratedCerts          map[string]string `json:"generatedCerts,omitempty"`
	IssuedAt                string            `json:"issuedAt,omitempty"`
	Issuer                  string            `json:"issuer,omitempty"`
	Key                     string            `json:"key,omitempty"`
	KeySize                 *int64            `json:"keySize,omitempty"`
	KnownIPs                []string          `json:"knownIps,omitempty"`
	Labels                  map[string]string `json:"labels,omitempty"`
	Mode                    string            `json:"mode,omitempty"`
	Name                    string            `json:"name,omitempty"`
	OwnerReferences         []OwnerReference  `json:"ownerReferences,omitempty"`
	Removed                 string            `json:"removed,omitempty"`
	SerialNumber            string            `json:"serialNumber,omitempty"`
	SubjectAlternativeNames []string          `json:"subjectAlternativeNames,omitempty"`
	TOS                     []string          `json:"tos,omitempty"`
	Uuid                    string            `json:"uuid,omitempty"`
	Version                 *int64            `json:"version,omitempty"`
}
type ListenConfigCollection struct {
	types.Collection
	Data   []ListenConfig `json:"data,omitempty"`
	client *ListenConfigClient
}

type ListenConfigClient struct {
	apiClient *Client
}

type ListenConfigOperations interface {
	List(opts *types.ListOpts) (*ListenConfigCollection, error)
	Create(opts *ListenConfig) (*ListenConfig, error)
	Update(existing *ListenConfig, updates interface{}) (*ListenConfig, error)
	ByID(id string) (*ListenConfig, error)
	Delete(container *ListenConfig) error
}

func newListenConfigClient(apiClient *Client) *ListenConfigClient {
	return &ListenConfigClient{
		apiClient: apiClient,
	}
}

func (c *ListenConfigClient) Create(container *ListenConfig) (*ListenConfig, error) {
	resp := &ListenConfig{}
	err := c.apiClient.Ops.DoCreate(ListenConfigType, container, resp)
	return resp, err
}

func (c *ListenConfigClient) Update(existing *ListenConfig, updates interface{}) (*ListenConfig, error) {
	resp := &ListenConfig{}
	err := c.apiClient.Ops.DoUpdate(ListenConfigType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ListenConfigClient) List(opts *types.ListOpts) (*ListenConfigCollection, error) {
	resp := &ListenConfigCollection{}
	err := c.apiClient.Ops.DoList(ListenConfigType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ListenConfigCollection) Next() (*ListenConfigCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ListenConfigCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ListenConfigClient) ByID(id string) (*ListenConfig, error) {
	resp := &ListenConfig{}
	err := c.apiClient.Ops.DoByID(ListenConfigType, id, resp)
	return resp, err
}

func (c *ListenConfigClient) Delete(container *ListenConfig) error {
	return c.apiClient.Ops.DoResourceDelete(ListenConfigType, &container.Resource)
}
