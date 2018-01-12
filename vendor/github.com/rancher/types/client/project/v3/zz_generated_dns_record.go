package client

import (
	"github.com/rancher/norman/types"
)

const (
	DNSRecordType                      = "dnsRecord"
	DNSRecordFieldAnnotations          = "annotations"
	DNSRecordFieldClusterIp            = "clusterIp"
	DNSRecordFieldCreated              = "created"
	DNSRecordFieldCreatorID            = "creatorId"
	DNSRecordFieldDescription          = "description"
	DNSRecordFieldHostname             = "hostname"
	DNSRecordFieldIPAddresses          = "ipAddresses"
	DNSRecordFieldLabels               = "labels"
	DNSRecordFieldName                 = "name"
	DNSRecordFieldNamespaceId          = "namespaceId"
	DNSRecordFieldOwnerReferences      = "ownerReferences"
	DNSRecordFieldProjectID            = "projectId"
	DNSRecordFieldRemoved              = "removed"
	DNSRecordFieldSelector             = "selector"
	DNSRecordFieldState                = "state"
	DNSRecordFieldTargetDNSRecordIDs   = "targetDnsRecordIds"
	DNSRecordFieldTargetWorkloadIDs    = "targetWorkloadIds"
	DNSRecordFieldTransitioning        = "transitioning"
	DNSRecordFieldTransitioningMessage = "transitioningMessage"
	DNSRecordFieldUuid                 = "uuid"
	DNSRecordFieldWorkloadID           = "workloadId"
)

type DNSRecord struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty"`
	ClusterIp            string            `json:"clusterIp,omitempty"`
	Created              string            `json:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty"`
	Description          string            `json:"description,omitempty"`
	Hostname             string            `json:"hostname,omitempty"`
	IPAddresses          []string          `json:"ipAddresses,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
	Name                 string            `json:"name,omitempty"`
	NamespaceId          string            `json:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty"`
	ProjectID            string            `json:"projectId,omitempty"`
	Removed              string            `json:"removed,omitempty"`
	Selector             map[string]string `json:"selector,omitempty"`
	State                string            `json:"state,omitempty"`
	TargetDNSRecordIDs   []string          `json:"targetDnsRecordIds,omitempty"`
	TargetWorkloadIDs    []string          `json:"targetWorkloadIds,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty"`
	Uuid                 string            `json:"uuid,omitempty"`
	WorkloadID           string            `json:"workloadId,omitempty"`
}
type DNSRecordCollection struct {
	types.Collection
	Data   []DNSRecord `json:"data,omitempty"`
	client *DNSRecordClient
}

type DNSRecordClient struct {
	apiClient *Client
}

type DNSRecordOperations interface {
	List(opts *types.ListOpts) (*DNSRecordCollection, error)
	Create(opts *DNSRecord) (*DNSRecord, error)
	Update(existing *DNSRecord, updates interface{}) (*DNSRecord, error)
	ByID(id string) (*DNSRecord, error)
	Delete(container *DNSRecord) error
}

func newDNSRecordClient(apiClient *Client) *DNSRecordClient {
	return &DNSRecordClient{
		apiClient: apiClient,
	}
}

func (c *DNSRecordClient) Create(container *DNSRecord) (*DNSRecord, error) {
	resp := &DNSRecord{}
	err := c.apiClient.Ops.DoCreate(DNSRecordType, container, resp)
	return resp, err
}

func (c *DNSRecordClient) Update(existing *DNSRecord, updates interface{}) (*DNSRecord, error) {
	resp := &DNSRecord{}
	err := c.apiClient.Ops.DoUpdate(DNSRecordType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *DNSRecordClient) List(opts *types.ListOpts) (*DNSRecordCollection, error) {
	resp := &DNSRecordCollection{}
	err := c.apiClient.Ops.DoList(DNSRecordType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *DNSRecordCollection) Next() (*DNSRecordCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &DNSRecordCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *DNSRecordClient) ByID(id string) (*DNSRecord, error) {
	resp := &DNSRecord{}
	err := c.apiClient.Ops.DoByID(DNSRecordType, id, resp)
	return resp, err
}

func (c *DNSRecordClient) Delete(container *DNSRecord) error {
	return c.apiClient.Ops.DoResourceDelete(DNSRecordType, &container.Resource)
}
