package client

import (
	"github.com/rancher/norman/types"
)

const (
	WorkloadType                               = "workload"
	WorkloadFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	WorkloadFieldAnnotations                   = "annotations"
	WorkloadFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	WorkloadFieldBatchSize                     = "batchSize"
	WorkloadFieldContainers                    = "containers"
	WorkloadFieldCreated                       = "created"
	WorkloadFieldCreatorID                     = "creatorId"
	WorkloadFieldDNSPolicy                     = "dnsPolicy"
	WorkloadFieldDeploymentStrategy            = "deploymentStrategy"
	WorkloadFieldDescription                   = "description"
	WorkloadFieldFsgid                         = "fsgid"
	WorkloadFieldGids                          = "gids"
	WorkloadFieldHostAliases                   = "hostAliases"
	WorkloadFieldHostname                      = "hostname"
	WorkloadFieldIPC                           = "ipc"
	WorkloadFieldLabels                        = "labels"
	WorkloadFieldName                          = "name"
	WorkloadFieldNamespaceId                   = "namespaceId"
	WorkloadFieldNet                           = "net"
	WorkloadFieldNodeId                        = "nodeId"
	WorkloadFieldOwnerReferences               = "ownerReferences"
	WorkloadFieldPID                           = "pid"
	WorkloadFieldPriority                      = "priority"
	WorkloadFieldPriorityClassName             = "priorityClassName"
	WorkloadFieldProjectID                     = "projectId"
	WorkloadFieldPullPolicy                    = "pullPolicy"
	WorkloadFieldPullSecrets                   = "pullSecrets"
	WorkloadFieldRemoved                       = "removed"
	WorkloadFieldRestart                       = "restart"
	WorkloadFieldRunAsNonRoot                  = "runAsNonRoot"
	WorkloadFieldScale                         = "scale"
	WorkloadFieldSchedulerName                 = "schedulerName"
	WorkloadFieldScheduling                    = "scheduling"
	WorkloadFieldServiceAccountName            = "serviceAccountName"
	WorkloadFieldServiceLinks                  = "serviceLinks"
	WorkloadFieldState                         = "state"
	WorkloadFieldStatus                        = "status"
	WorkloadFieldSubdomain                     = "subdomain"
	WorkloadFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	WorkloadFieldTransitioning                 = "transitioning"
	WorkloadFieldTransitioningMessage          = "transitioningMessage"
	WorkloadFieldUid                           = "uid"
	WorkloadFieldUuid                          = "uuid"
	WorkloadFieldVolumes                       = "volumes"
	WorkloadFieldWorkloadAnnotations           = "workloadAnnotations"
	WorkloadFieldWorkloadLabels                = "workloadLabels"
)

type Workload struct {
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
	Description                   string                 `json:"description,omitempty"`
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
	ServiceLinks                  []Link                 `json:"serviceLinks,omitempty"`
	State                         string                 `json:"state,omitempty"`
	Status                        *WorkloadStatus        `json:"status,omitempty"`
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
type WorkloadCollection struct {
	types.Collection
	Data   []Workload `json:"data,omitempty"`
	client *WorkloadClient
}

type WorkloadClient struct {
	apiClient *Client
}

type WorkloadOperations interface {
	List(opts *types.ListOpts) (*WorkloadCollection, error)
	Create(opts *Workload) (*Workload, error)
	Update(existing *Workload, updates interface{}) (*Workload, error)
	ByID(id string) (*Workload, error)
	Delete(container *Workload) error
}

func newWorkloadClient(apiClient *Client) *WorkloadClient {
	return &WorkloadClient{
		apiClient: apiClient,
	}
}

func (c *WorkloadClient) Create(container *Workload) (*Workload, error) {
	resp := &Workload{}
	err := c.apiClient.Ops.DoCreate(WorkloadType, container, resp)
	return resp, err
}

func (c *WorkloadClient) Update(existing *Workload, updates interface{}) (*Workload, error) {
	resp := &Workload{}
	err := c.apiClient.Ops.DoUpdate(WorkloadType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *WorkloadClient) List(opts *types.ListOpts) (*WorkloadCollection, error) {
	resp := &WorkloadCollection{}
	err := c.apiClient.Ops.DoList(WorkloadType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *WorkloadCollection) Next() (*WorkloadCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &WorkloadCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *WorkloadClient) ByID(id string) (*Workload, error) {
	resp := &Workload{}
	err := c.apiClient.Ops.DoByID(WorkloadType, id, resp)
	return resp, err
}

func (c *WorkloadClient) Delete(container *Workload) error {
	return c.apiClient.Ops.DoResourceDelete(WorkloadType, &container.Resource)
}
