package client

import (
	"github.com/rancher/norman/types"
)

const (
	ServiceType                          = "service"
	ServiceFieldAnnotations              = "annotations"
	ServiceFieldClusterIp                = "clusterIp"
	ServiceFieldCreated                  = "created"
	ServiceFieldCreatorID                = "creatorId"
	ServiceFieldDescription              = "description"
	ServiceFieldExternalIPs              = "externalIPs"
	ServiceFieldExternalTrafficPolicy    = "externalTrafficPolicy"
	ServiceFieldHealthCheckNodePort      = "healthCheckNodePort"
	ServiceFieldHostname                 = "hostname"
	ServiceFieldIPAddresses              = "ipAddresses"
	ServiceFieldKind                     = "kind"
	ServiceFieldLabels                   = "labels"
	ServiceFieldLoadBalancerIP           = "loadBalancerIP"
	ServiceFieldLoadBalancerSourceRanges = "loadBalancerSourceRanges"
	ServiceFieldName                     = "name"
	ServiceFieldNamespaceId              = "namespaceId"
	ServiceFieldOwnerReferences          = "ownerReferences"
	ServiceFieldPorts                    = "ports"
	ServiceFieldProjectID                = "projectId"
	ServiceFieldPublicEndpoints          = "publicEndpoints"
	ServiceFieldPublishNotReadyAddresses = "publishNotReadyAddresses"
	ServiceFieldRemoved                  = "removed"
	ServiceFieldSelector                 = "selector"
	ServiceFieldSessionAffinity          = "sessionAffinity"
	ServiceFieldSessionAffinityConfig    = "sessionAffinityConfig"
	ServiceFieldState                    = "state"
	ServiceFieldTargetDNSRecordIDs       = "targetDnsRecordIds"
	ServiceFieldTargetWorkloadIDs        = "targetWorkloadIds"
	ServiceFieldTransitioning            = "transitioning"
	ServiceFieldTransitioningMessage     = "transitioningMessage"
	ServiceFieldUuid                     = "uuid"
	ServiceFieldWorkloadID               = "workloadId"
)

type Service struct {
	types.Resource
	Annotations              map[string]string      `json:"annotations,omitempty"`
	ClusterIp                string                 `json:"clusterIp,omitempty"`
	Created                  string                 `json:"created,omitempty"`
	CreatorID                string                 `json:"creatorId,omitempty"`
	Description              string                 `json:"description,omitempty"`
	ExternalIPs              []string               `json:"externalIPs,omitempty"`
	ExternalTrafficPolicy    string                 `json:"externalTrafficPolicy,omitempty"`
	HealthCheckNodePort      *int64                 `json:"healthCheckNodePort,omitempty"`
	Hostname                 string                 `json:"hostname,omitempty"`
	IPAddresses              []string               `json:"ipAddresses,omitempty"`
	Kind                     string                 `json:"kind,omitempty"`
	Labels                   map[string]string      `json:"labels,omitempty"`
	LoadBalancerIP           string                 `json:"loadBalancerIP,omitempty"`
	LoadBalancerSourceRanges []string               `json:"loadBalancerSourceRanges,omitempty"`
	Name                     string                 `json:"name,omitempty"`
	NamespaceId              string                 `json:"namespaceId,omitempty"`
	OwnerReferences          []OwnerReference       `json:"ownerReferences,omitempty"`
	Ports                    []ServicePort          `json:"ports,omitempty"`
	ProjectID                string                 `json:"projectId,omitempty"`
	PublicEndpoints          []PublicEndpoint       `json:"publicEndpoints,omitempty"`
	PublishNotReadyAddresses bool                   `json:"publishNotReadyAddresses,omitempty"`
	Removed                  string                 `json:"removed,omitempty"`
	Selector                 map[string]string      `json:"selector,omitempty"`
	SessionAffinity          string                 `json:"sessionAffinity,omitempty"`
	SessionAffinityConfig    *SessionAffinityConfig `json:"sessionAffinityConfig,omitempty"`
	State                    string                 `json:"state,omitempty"`
	TargetDNSRecordIDs       []string               `json:"targetDnsRecordIds,omitempty"`
	TargetWorkloadIDs        []string               `json:"targetWorkloadIds,omitempty"`
	Transitioning            string                 `json:"transitioning,omitempty"`
	TransitioningMessage     string                 `json:"transitioningMessage,omitempty"`
	Uuid                     string                 `json:"uuid,omitempty"`
	WorkloadID               string                 `json:"workloadId,omitempty"`
}
type ServiceCollection struct {
	types.Collection
	Data   []Service `json:"data,omitempty"`
	client *ServiceClient
}

type ServiceClient struct {
	apiClient *Client
}

type ServiceOperations interface {
	List(opts *types.ListOpts) (*ServiceCollection, error)
	Create(opts *Service) (*Service, error)
	Update(existing *Service, updates interface{}) (*Service, error)
	ByID(id string) (*Service, error)
	Delete(container *Service) error
}

func newServiceClient(apiClient *Client) *ServiceClient {
	return &ServiceClient{
		apiClient: apiClient,
	}
}

func (c *ServiceClient) Create(container *Service) (*Service, error) {
	resp := &Service{}
	err := c.apiClient.Ops.DoCreate(ServiceType, container, resp)
	return resp, err
}

func (c *ServiceClient) Update(existing *Service, updates interface{}) (*Service, error) {
	resp := &Service{}
	err := c.apiClient.Ops.DoUpdate(ServiceType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ServiceClient) List(opts *types.ListOpts) (*ServiceCollection, error) {
	resp := &ServiceCollection{}
	err := c.apiClient.Ops.DoList(ServiceType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ServiceCollection) Next() (*ServiceCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ServiceCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ServiceClient) ByID(id string) (*Service, error) {
	resp := &Service{}
	err := c.apiClient.Ops.DoByID(ServiceType, id, resp)
	return resp, err
}

func (c *ServiceClient) Delete(container *Service) error {
	return c.apiClient.Ops.DoResourceDelete(ServiceType, &container.Resource)
}
