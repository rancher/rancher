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
	DNSRecordFieldIPFamily             = "ipFamily"
	DNSRecordFieldLabels               = "labels"
	DNSRecordFieldName                 = "name"
	DNSRecordFieldNamespaceId          = "namespaceId"
	DNSRecordFieldOwnerReferences      = "ownerReferences"
	DNSRecordFieldPorts                = "ports"
	DNSRecordFieldProjectID            = "projectId"
	DNSRecordFieldPublicEndpoints      = "publicEndpoints"
	DNSRecordFieldRemoved              = "removed"
	DNSRecordFieldSelector             = "selector"
	DNSRecordFieldState                = "state"
	DNSRecordFieldTargetDNSRecordIDs   = "targetDnsRecordIds"
	DNSRecordFieldTargetWorkloadIDs    = "targetWorkloadIds"
	DNSRecordFieldTopologyKeys         = "topologyKeys"
	DNSRecordFieldTransitioning        = "transitioning"
	DNSRecordFieldTransitioningMessage = "transitioningMessage"
	DNSRecordFieldUUID                 = "uuid"
	DNSRecordFieldWorkloadID           = "workloadId"
)

type DNSRecord struct {
	types.Resource
	Annotations          map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	ClusterIp            string            `json:"clusterIp,omitempty" yaml:"clusterIp,omitempty"`
	Created              string            `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID            string            `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	Description          string            `json:"description,omitempty" yaml:"description,omitempty"`
	Hostname             string            `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	IPAddresses          []string          `json:"ipAddresses,omitempty" yaml:"ipAddresses,omitempty"`
	IPFamily             string            `json:"ipFamily,omitempty" yaml:"ipFamily,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                 string            `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId          string            `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	OwnerReferences      []OwnerReference  `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Ports                []ServicePort     `json:"ports,omitempty" yaml:"ports,omitempty"`
	ProjectID            string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	PublicEndpoints      []PublicEndpoint  `json:"publicEndpoints,omitempty" yaml:"publicEndpoints,omitempty"`
	Removed              string            `json:"removed,omitempty" yaml:"removed,omitempty"`
	Selector             map[string]string `json:"selector,omitempty" yaml:"selector,omitempty"`
	State                string            `json:"state,omitempty" yaml:"state,omitempty"`
	TargetDNSRecordIDs   []string          `json:"targetDnsRecordIds,omitempty" yaml:"targetDnsRecordIds,omitempty"`
	TargetWorkloadIDs    []string          `json:"targetWorkloadIds,omitempty" yaml:"targetWorkloadIds,omitempty"`
	TopologyKeys         []string          `json:"topologyKeys,omitempty" yaml:"topologyKeys,omitempty"`
	Transitioning        string            `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage string            `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                 string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	WorkloadID           string            `json:"workloadId,omitempty" yaml:"workloadId,omitempty"`
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
	Replace(existing *DNSRecord) (*DNSRecord, error)
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

func (c *DNSRecordClient) Replace(obj *DNSRecord) (*DNSRecord, error) {
	resp := &DNSRecord{}
	err := c.apiClient.Ops.DoReplace(DNSRecordType, &obj.Resource, obj, resp)
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
