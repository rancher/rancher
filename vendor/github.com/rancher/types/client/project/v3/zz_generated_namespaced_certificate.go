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
	NamespacedCertificateFieldExpiresAt               = "expiresAt"
	NamespacedCertificateFieldFinalizers              = "finalizers"
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
	NamespacedCertificateFieldUuid                    = "uuid"
	NamespacedCertificateFieldVersion                 = "version"
)

type NamespacedCertificate struct {
	types.Resource
	Algorithm               string            `json:"algorithm,omitempty"`
	Annotations             map[string]string `json:"annotations,omitempty"`
	CN                      string            `json:"cn,omitempty"`
	CertFingerprint         string            `json:"certFingerprint,omitempty"`
	Certs                   string            `json:"certs,omitempty"`
	Created                 string            `json:"created,omitempty"`
	CreatorID               string            `json:"creatorId,omitempty"`
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
	SubjectAlternativeNames string            `json:"subjectAlternativeNames,omitempty"`
	Uuid                    string            `json:"uuid,omitempty"`
	Version                 string            `json:"version,omitempty"`
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
