package client

import (
	"github.com/rancher/norman/types"
)

const (
	ReplicaSetType                               = "replicaSet"
	ReplicaSetFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	ReplicaSetFieldAnnotations                   = "annotations"
	ReplicaSetFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	ReplicaSetFieldBatchSize                     = "batchSize"
	ReplicaSetFieldContainers                    = "containers"
	ReplicaSetFieldCreated                       = "created"
	ReplicaSetFieldCreatorID                     = "creatorId"
	ReplicaSetFieldDNSPolicy                     = "dnsPolicy"
	ReplicaSetFieldDeploymentStrategy            = "deploymentStrategy"
	ReplicaSetFieldFsgid                         = "fsgid"
	ReplicaSetFieldGids                          = "gids"
	ReplicaSetFieldHostAliases                   = "hostAliases"
	ReplicaSetFieldHostname                      = "hostname"
	ReplicaSetFieldIPC                           = "ipc"
	ReplicaSetFieldLabels                        = "labels"
	ReplicaSetFieldName                          = "name"
	ReplicaSetFieldNamespaceId                   = "namespaceId"
	ReplicaSetFieldNet                           = "net"
	ReplicaSetFieldNodeId                        = "nodeId"
	ReplicaSetFieldOwnerReferences               = "ownerReferences"
	ReplicaSetFieldPID                           = "pid"
	ReplicaSetFieldPriority                      = "priority"
	ReplicaSetFieldPriorityClassName             = "priorityClassName"
	ReplicaSetFieldProjectID                     = "projectId"
	ReplicaSetFieldPullPolicy                    = "pullPolicy"
	ReplicaSetFieldPullSecrets                   = "pullSecrets"
	ReplicaSetFieldRemoved                       = "removed"
	ReplicaSetFieldRestart                       = "restart"
	ReplicaSetFieldRunAsNonRoot                  = "runAsNonRoot"
	ReplicaSetFieldScale                         = "scale"
	ReplicaSetFieldSchedulerName                 = "schedulerName"
	ReplicaSetFieldScheduling                    = "scheduling"
	ReplicaSetFieldServiceAccountName            = "serviceAccountName"
	ReplicaSetFieldState                         = "state"
	ReplicaSetFieldStatus                        = "status"
	ReplicaSetFieldSubdomain                     = "subdomain"
	ReplicaSetFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	ReplicaSetFieldTransitioning                 = "transitioning"
	ReplicaSetFieldTransitioningMessage          = "transitioningMessage"
	ReplicaSetFieldUid                           = "uid"
	ReplicaSetFieldUuid                          = "uuid"
	ReplicaSetFieldVolumes                       = "volumes"
	ReplicaSetFieldWorkloadAnnotations           = "workloadAnnotations"
	ReplicaSetFieldWorkloadLabels                = "workloadLabels"
)

type ReplicaSet struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string      `json:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	BatchSize                     string                 `json:"batchSize,omitempty"`
	Containers                    []Container            `json:"containers,omitempty"`
	Created                       string                 `json:"created,omitempty"`
	CreatorID                     string                 `json:"creatorId,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	DeploymentStrategy            *DeployStrategy        `json:"deploymentStrategy,omitempty"`
	Fsgid                         *int64                 `json:"fsgid,omitempty"`
	Gids                          []int64                `json:"gids,omitempty"`
	HostAliases                   map[string]HostAlias   `json:"hostAliases,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty"`
	IPC                           string                 `json:"ipc,omitempty"`
	Labels                        map[string]string      `json:"labels,omitempty"`
	Name                          string                 `json:"name,omitempty"`
	NamespaceId                   string                 `json:"namespaceId,omitempty"`
	Net                           string                 `json:"net,omitempty"`
	NodeId                        string                 `json:"nodeId,omitempty"`
	OwnerReferences               []OwnerReference       `json:"ownerReferences,omitempty"`
	PID                           string                 `json:"pid,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty"`
	ProjectID                     string                 `json:"projectId,omitempty"`
	PullPolicy                    string                 `json:"pullPolicy,omitempty"`
	PullSecrets                   []LocalObjectReference `json:"pullSecrets,omitempty"`
	Removed                       string                 `json:"removed,omitempty"`
	Restart                       string                 `json:"restart,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty"`
	Scale                         *int64                 `json:"scale,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty"`
	State                         string                 `json:"state,omitempty"`
	Status                        *ReplicaSetStatus      `json:"status,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Transitioning                 string                 `json:"transitioning,omitempty"`
	TransitioningMessage          string                 `json:"transitioningMessage,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Uuid                          string                 `json:"uuid,omitempty"`
	Volumes                       map[string]Volume      `json:"volumes,omitempty"`
	WorkloadAnnotations           map[string]string      `json:"workloadAnnotations,omitempty"`
	WorkloadLabels                map[string]string      `json:"workloadLabels,omitempty"`
}
type ReplicaSetCollection struct {
	types.Collection
	Data   []ReplicaSet `json:"data,omitempty"`
	client *ReplicaSetClient
}

type ReplicaSetClient struct {
	apiClient *Client
}

type ReplicaSetOperations interface {
	List(opts *types.ListOpts) (*ReplicaSetCollection, error)
	Create(opts *ReplicaSet) (*ReplicaSet, error)
	Update(existing *ReplicaSet, updates interface{}) (*ReplicaSet, error)
	ByID(id string) (*ReplicaSet, error)
	Delete(container *ReplicaSet) error
}

func newReplicaSetClient(apiClient *Client) *ReplicaSetClient {
	return &ReplicaSetClient{
		apiClient: apiClient,
	}
}

func (c *ReplicaSetClient) Create(container *ReplicaSet) (*ReplicaSet, error) {
	resp := &ReplicaSet{}
	err := c.apiClient.Ops.DoCreate(ReplicaSetType, container, resp)
	return resp, err
}

func (c *ReplicaSetClient) Update(existing *ReplicaSet, updates interface{}) (*ReplicaSet, error) {
	resp := &ReplicaSet{}
	err := c.apiClient.Ops.DoUpdate(ReplicaSetType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ReplicaSetClient) List(opts *types.ListOpts) (*ReplicaSetCollection, error) {
	resp := &ReplicaSetCollection{}
	err := c.apiClient.Ops.DoList(ReplicaSetType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ReplicaSetCollection) Next() (*ReplicaSetCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ReplicaSetCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ReplicaSetClient) ByID(id string) (*ReplicaSet, error) {
	resp := &ReplicaSet{}
	err := c.apiClient.Ops.DoByID(ReplicaSetType, id, resp)
	return resp, err
}

func (c *ReplicaSetClient) Delete(container *ReplicaSet) error {
	return c.apiClient.Ops.DoResourceDelete(ReplicaSetType, &container.Resource)
}
