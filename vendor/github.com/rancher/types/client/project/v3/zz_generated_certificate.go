package client

import (
	"github.com/rancher/norman/types"
)

const (
	CertificateType                         = "certificate"
	CertificateFieldAlgorithm               = "algorithm"
	CertificateFieldAnnotations             = "annotations"
	CertificateFieldCN                      = "cn"
	CertificateFieldCertFingerprint         = "certFingerprint"
	CertificateFieldCerts                   = "certs"
	CertificateFieldCreated                 = "created"
	CertificateFieldCreatorID               = "creatorId"
	CertificateFieldDescription             = "description"
	CertificateFieldExpiresAt               = "expiresAt"
	CertificateFieldFinalizers              = "finalizers"
	CertificateFieldIssuedAt                = "issuedAt"
	CertificateFieldIssuer                  = "issuer"
	CertificateFieldKey                     = "key"
	CertificateFieldKeySize                 = "keySize"
	CertificateFieldLabels                  = "labels"
	CertificateFieldName                    = "name"
	CertificateFieldNamespaceId             = "namespaceId"
	CertificateFieldOwnerReferences         = "ownerReferences"
	CertificateFieldProjectID               = "projectId"
	CertificateFieldRemoved                 = "removed"
	CertificateFieldSerialNumber            = "serialNumber"
	CertificateFieldSubjectAlternativeNames = "subjectAlternativeNames"
	CertificateFieldUuid                    = "uuid"
	CertificateFieldVersion                 = "version"
)

type Certificate struct {
	types.Resource
	Algorithm               string            `json:"algorithm,omitempty"`
	Annotations             map[string]string `json:"annotations,omitempty"`
	CN                      string            `json:"cn,omitempty"`
	CertFingerprint         string            `json:"certFingerprint,omitempty"`
	Certs                   string            `json:"certs,omitempty"`
	Created                 string            `json:"created,omitempty"`
	CreatorID               string            `json:"creatorId,omitempty"`
	Description             string            `json:"description,omitempty"`
	ExpiresAt               string            `json:"expiresAt,omitempty"`
	Finalizers              []string          `json:"finalizers,omitempty"`
	IssuedAt                string            `json:"issuedAt,omitempty"`
	Issuer                  string            `json:"issuer,omitempty"`
	Key                     string            `json:"key,omitempty"`
	KeySize                 string            `json:"keySize,omitempty"`
	Labels                  map[string]string `json:"labels,omitempty"`
	Name                    string            `json:"name,omitempty"`
	NamespaceId             string            `json:"namespaceId,omitempty"`
	OwnerReferences         []OwnerReference  `json:"ownerReferences,omitempty"`
	ProjectID               string            `json:"projectId,omitempty"`
	Removed                 string            `json:"removed,omitempty"`
	SerialNumber            string            `json:"serialNumber,omitempty"`
	SubjectAlternativeNames []string          `json:"subjectAlternativeNames,omitempty"`
	Uuid                    string            `json:"uuid,omitempty"`
	Version                 string            `json:"version,omitempty"`
}
type CertificateCollection struct {
	types.Collection
	Data   []Certificate `json:"data,omitempty"`
	client *CertificateClient
}

type CertificateClient struct {
	apiClient *Client
}

type CertificateOperations interface {
	List(opts *types.ListOpts) (*CertificateCollection, error)
	Create(opts *Certificate) (*Certificate, error)
	Update(existing *Certificate, updates interface{}) (*Certificate, error)
	ByID(id string) (*Certificate, error)
	Delete(container *Certificate) error
}

func newCertificateClient(apiClient *Client) *CertificateClient {
	return &CertificateClient{
		apiClient: apiClient,
	}
}

func (c *CertificateClient) Create(container *Certificate) (*Certificate, error) {
	resp := &Certificate{}
	err := c.apiClient.Ops.DoCreate(CertificateType, container, resp)
	return resp, err
}

func (c *CertificateClient) Update(existing *Certificate, updates interface{}) (*Certificate, error) {
	resp := &Certificate{}
	err := c.apiClient.Ops.DoUpdate(CertificateType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *CertificateClient) List(opts *types.ListOpts) (*CertificateCollection, error) {
	resp := &CertificateCollection{}
	err := c.apiClient.Ops.DoList(CertificateType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *CertificateCollection) Next() (*CertificateCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &CertificateCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *CertificateClient) ByID(id string) (*Certificate, error) {
	resp := &Certificate{}
	err := c.apiClient.Ops.DoByID(CertificateType, id, resp)
	return resp, err
}

func (c *CertificateClient) Delete(container *Certificate) error {
	return c.apiClient.Ops.DoResourceDelete(CertificateType, &container.Resource)
}
