package client

import (
	"github.com/rancher/norman/types"
)

const (
	ReplicationControllerType                               = "replicationController"
	ReplicationControllerFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	ReplicationControllerFieldAnnotations                   = "annotations"
	ReplicationControllerFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	ReplicationControllerFieldContainers                    = "containers"
	ReplicationControllerFieldCreated                       = "created"
	ReplicationControllerFieldCreatorID                     = "creatorId"
	ReplicationControllerFieldDNSConfig                     = "dnsConfig"
	ReplicationControllerFieldDNSPolicy                     = "dnsPolicy"
	ReplicationControllerFieldFsgid                         = "fsgid"
	ReplicationControllerFieldGids                          = "gids"
	ReplicationControllerFieldHostAliases                   = "hostAliases"
	ReplicationControllerFieldHostIPC                       = "hostIPC"
	ReplicationControllerFieldHostNetwork                   = "hostNetwork"
	ReplicationControllerFieldHostPID                       = "hostPID"
	ReplicationControllerFieldHostname                      = "hostname"
	ReplicationControllerFieldImagePullSecrets              = "imagePullSecrets"
	ReplicationControllerFieldLabels                        = "labels"
	ReplicationControllerFieldName                          = "name"
	ReplicationControllerFieldNamespaceId                   = "namespaceId"
	ReplicationControllerFieldNodeId                        = "nodeId"
	ReplicationControllerFieldOwnerReferences               = "ownerReferences"
	ReplicationControllerFieldPriority                      = "priority"
	ReplicationControllerFieldPriorityClassName             = "priorityClassName"
	ReplicationControllerFieldProjectID                     = "projectId"
	ReplicationControllerFieldPublicEndpoints               = "publicEndpoints"
	ReplicationControllerFieldRemoved                       = "removed"
	ReplicationControllerFieldReplicationControllerConfig   = "replicationControllerConfig"
	ReplicationControllerFieldReplicationControllerStatus   = "replicationControllerStatus"
	ReplicationControllerFieldRestartPolicy                 = "restartPolicy"
	ReplicationControllerFieldRunAsNonRoot                  = "runAsNonRoot"
	ReplicationControllerFieldScale                         = "scale"
	ReplicationControllerFieldSchedulerName                 = "schedulerName"
	ReplicationControllerFieldScheduling                    = "scheduling"
	ReplicationControllerFieldSelector                      = "selector"
	ReplicationControllerFieldServiceAccountName            = "serviceAccountName"
	ReplicationControllerFieldState                         = "state"
	ReplicationControllerFieldSubdomain                     = "subdomain"
	ReplicationControllerFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	ReplicationControllerFieldTransitioning                 = "transitioning"
	ReplicationControllerFieldTransitioningMessage          = "transitioningMessage"
	ReplicationControllerFieldUid                           = "uid"
	ReplicationControllerFieldUuid                          = "uuid"
	ReplicationControllerFieldVolumes                       = "volumes"
	ReplicationControllerFieldWorkloadAnnotations           = "workloadAnnotations"
	ReplicationControllerFieldWorkloadLabels                = "workloadLabels"
)

type ReplicationController struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                       `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string            `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                        `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	Containers                    []Container                  `json:"containers,omitempty" yaml:"containers,omitempty"`
	Created                       string                       `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                     string                       `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DNSConfig                     *PodDNSConfig                `json:"dnsConfig,omitempty" yaml:"dnsConfig,omitempty"`
	DNSPolicy                     string                       `json:"dnsPolicy,omitempty" yaml:"dnsPolicy,omitempty"`
	Fsgid                         *int64                       `json:"fsgid,omitempty" yaml:"fsgid,omitempty"`
	Gids                          []int64                      `json:"gids,omitempty" yaml:"gids,omitempty"`
	HostAliases                   []HostAlias                  `json:"hostAliases,omitempty" yaml:"hostAliases,omitempty"`
	HostIPC                       bool                         `json:"hostIPC,omitempty" yaml:"hostIPC,omitempty"`
	HostNetwork                   bool                         `json:"hostNetwork,omitempty" yaml:"hostNetwork,omitempty"`
	HostPID                       bool                         `json:"hostPID,omitempty" yaml:"hostPID,omitempty"`
	Hostname                      string                       `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference       `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	Labels                        map[string]string            `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                          string                       `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId                   string                       `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NodeId                        string                       `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	OwnerReferences               []OwnerReference             `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Priority                      *int64                       `json:"priority,omitempty" yaml:"priority,omitempty"`
	PriorityClassName             string                       `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	ProjectID                     string                       `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	PublicEndpoints               []PublicEndpoint             `json:"publicEndpoints,omitempty" yaml:"publicEndpoints,omitempty"`
	Removed                       string                       `json:"removed,omitempty" yaml:"removed,omitempty"`
	ReplicationControllerConfig   *ReplicationControllerConfig `json:"replicationControllerConfig,omitempty" yaml:"replicationControllerConfig,omitempty"`
	ReplicationControllerStatus   *ReplicationControllerStatus `json:"replicationControllerStatus,omitempty" yaml:"replicationControllerStatus,omitempty"`
	RestartPolicy                 string                       `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsNonRoot                  *bool                        `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	Scale                         *int64                       `json:"scale,omitempty" yaml:"scale,omitempty"`
	SchedulerName                 string                       `json:"schedulerName,omitempty" yaml:"schedulerName,omitempty"`
	Scheduling                    *Scheduling                  `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	Selector                      map[string]string            `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceAccountName            string                       `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	State                         string                       `json:"state,omitempty" yaml:"state,omitempty"`
	Subdomain                     string                       `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                       `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	Transitioning                 string                       `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage          string                       `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	Uid                           *int64                       `json:"uid,omitempty" yaml:"uid,omitempty"`
	Uuid                          string                       `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Volumes                       []Volume                     `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WorkloadAnnotations           map[string]string            `json:"workloadAnnotations,omitempty" yaml:"workloadAnnotations,omitempty"`
	WorkloadLabels                map[string]string            `json:"workloadLabels,omitempty" yaml:"workloadLabels,omitempty"`
}
type ReplicationControllerCollection struct {
	types.Collection
	Data   []ReplicationController `json:"data,omitempty"`
	client *ReplicationControllerClient
}

type ReplicationControllerClient struct {
	apiClient *Client
}

type ReplicationControllerOperations interface {
	List(opts *types.ListOpts) (*ReplicationControllerCollection, error)
	Create(opts *ReplicationController) (*ReplicationController, error)
	Update(existing *ReplicationController, updates interface{}) (*ReplicationController, error)
	ByID(id string) (*ReplicationController, error)
	Delete(container *ReplicationController) error
}

func newReplicationControllerClient(apiClient *Client) *ReplicationControllerClient {
	return &ReplicationControllerClient{
		apiClient: apiClient,
	}
}

func (c *ReplicationControllerClient) Create(container *ReplicationController) (*ReplicationController, error) {
	resp := &ReplicationController{}
	err := c.apiClient.Ops.DoCreate(ReplicationControllerType, container, resp)
	return resp, err
}

func (c *ReplicationControllerClient) Update(existing *ReplicationController, updates interface{}) (*ReplicationController, error) {
	resp := &ReplicationController{}
	err := c.apiClient.Ops.DoUpdate(ReplicationControllerType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ReplicationControllerClient) List(opts *types.ListOpts) (*ReplicationControllerCollection, error) {
	resp := &ReplicationControllerCollection{}
	err := c.apiClient.Ops.DoList(ReplicationControllerType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ReplicationControllerCollection) Next() (*ReplicationControllerCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ReplicationControllerCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ReplicationControllerClient) ByID(id string) (*ReplicationController, error) {
	resp := &ReplicationController{}
	err := c.apiClient.Ops.DoByID(ReplicationControllerType, id, resp)
	return resp, err
}

func (c *ReplicationControllerClient) Delete(container *ReplicationController) error {
	return c.apiClient.Ops.DoResourceDelete(ReplicationControllerType, &container.Resource)
}
