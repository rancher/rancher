package client

import (
	"github.com/rancher/norman/types"
)

const (
	NamespacedCertificateType                         = "namespacedCertificate"
	NamespacedCertificateFieldAlgorithm               = "algorithm"
	NamespacedCertificateFieldAnnotations             = "annotations"
	NamespacedCertificateFieldCN                      = "cn"
	NamespacedCertificateFieldCertFingerprint         = "certFingerprint"
	NamespacedCertificateFieldCerts                   = "certs"
	NamespacedCertificateFieldCreated                 = "created"
	NamespacedCertificateFieldCreatorID               = "creatorId"
	NamespacedCertificateFieldDescription             = "description"
	NamespacedCertificateFieldExpiresAt               = "expiresAt"
	NamespacedCertificateFieldIssuedAt                = "issuedAt"
	NamespacedCertificateFieldIssuer                  = "issuer"
	NamespacedCertificateFieldKey                     = "key"
	NamespacedCertificateFieldKeySize                 = "keySize"
	NamespacedCertificateFieldLabels                  = "labels"
	NamespacedCertificateFieldName                    = "name"
	NamespacedCertificateFieldNamespaceId             = "namespaceId"
	NamespacedCertificateFieldOwnerReferences         = "ownerReferences"
	NamespacedCertificateFieldProjectID               = "projectId"
	NamespacedCertificateFieldRemoved                 = "removed"
	NamespacedCertificateFieldSerialNumber            = "serialNumber"
	NamespacedCertificateFieldSubjectAlternativeNames = "subjectAlternativeNames"
	NamespacedCertificateFieldUUID                    = "uuid"
	NamespacedCertificateFieldVersion                 = "version"
)

type NamespacedCertificate struct {
	types.Resource
	Algorithm               string            `json:"algorithm,omitempty" yaml:"algorithm,omitempty"`
	Annotations             map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	CN                      string            `json:"cn,omitempty" yaml:"cn,omitempty"`
	CertFingerprint         string            `json:"certFingerprint,omitempty" yaml:"certFingerprint,omitempty"`
	Certs                   string            `json:"certs,omitempty" yaml:"certs,omitempty"`
	Created                 string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID               string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description             string            `json:"description,omitempty" yaml:"description,omitempty"`
	ExpiresAt               string            `json:"expiresAt,omitempty" yaml:"expiresAt,omitempty"`
	IssuedAt                string            `json:"issuedAt,omitempty" yaml:"issuedAt,omitempty"`
	Issuer                  string            `json:"issuer,omitempty" yaml:"issuer,omitempty"`
	Key                     string            `json:"key,omitempty" yaml:"key,omitempty"`
	KeySize                 string            `json:"keySize,omitempty" yaml:"keySize,omitempty"`
	Labels                  map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                    string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId             string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences         []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	ProjectID               string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	Removed                 string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	SerialNumber            string            `json:"serialNumber,omitempty" yaml:"serialNumber,omitempty"`
	SubjectAlternativeNames []string          `json:"subjectAlternativeNames,omitempty" yaml:"subjectAlternativeNames,omitempty"`
	UUID                    string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Version                 string            `json:"version,omitempty" yaml:"version,omitempty"`
}

type NamespacedCertificateCollection struct {
	types.Collection
	Data   []NamespacedCertificate `json:"data,omitempty"`
	client *NamespacedCertificateClient
}

type NamespacedCertificateClient struct {
	apiClient *Client
}

type NamespacedCertificateOperations interface {
	List(opts *types.ListOpts) (*NamespacedCertificateCollection, error)
	Create(opts *NamespacedCertificate) (*NamespacedCertificate, error)
	Update(existing *NamespacedCertificate, updates interface{}) (*NamespacedCertificate, error)
	Replace(existing *NamespacedCertificate) (*NamespacedCertificate, error)
	ByID(id string) (*NamespacedCertificate, error)
	Delete(container *NamespacedCertificate) error
}

func newNamespacedCertificateClient(apiClient *Client) *NamespacedCertificateClient {
	return &NamespacedCertificateClient{
		apiClient: apiClient,
	}
}

func (c *NamespacedCertificateClient) Create(container *NamespacedCertificate) (*NamespacedCertificate, error) {
	resp := &NamespacedCertificate{}
	err := c.apiClient.Ops.DoCreate(NamespacedCertificateType, container, resp)
	return resp, err
}

func (c *NamespacedCertificateClient) Update(existing *NamespacedCertificate, updates interface{}) (*NamespacedCertificate, error) {
	resp := &NamespacedCertificate{}
	err := c.apiClient.Ops.DoUpdate(NamespacedCertificateType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *NamespacedCertificateClient) Replace(obj *NamespacedCertificate) (*NamespacedCertificate, error) {
	resp := &NamespacedCertificate{}
	err := c.apiClient.Ops.DoReplace(NamespacedCertificateType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *NamespacedCertificateClient) List(opts *types.ListOpts) (*NamespacedCertificateCollection, error) {
	resp := &NamespacedCertificateCollection{}
	err := c.apiClient.Ops.DoList(NamespacedCertificateType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *NamespacedCertificateCollection) Next() (*NamespacedCertificateCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &NamespacedCertificateCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *NamespacedCertificateClient) ByID(id string) (*NamespacedCertificate, error) {
	resp := &NamespacedCertificate{}
	err := c.apiClient.Ops.DoByID(NamespacedCertificateType, id, resp)
	return resp, err
}

func (c *NamespacedCertificateClient) Delete(container *NamespacedCertificate) error {
	return c.apiClient.Ops.DoResourceDelete(NamespacedCertificateType, &container.Resource)
}
