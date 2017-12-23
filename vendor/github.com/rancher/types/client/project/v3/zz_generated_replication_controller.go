package client

import (
	"github.com/rancher/norman/types"
)

const (
	ReplicationControllerType                               = "replicationController"
	ReplicationControllerFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	ReplicationControllerFieldAnnotations                   = "annotations"
	ReplicationControllerFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	ReplicationControllerFieldBatchSize                     = "batchSize"
	ReplicationControllerFieldContainers                    = "containers"
	ReplicationControllerFieldCreated                       = "created"
	ReplicationControllerFieldDNSPolicy                     = "dnsPolicy"
	ReplicationControllerFieldDeploymentStrategy            = "deploymentStrategy"
	ReplicationControllerFieldFinalizers                    = "finalizers"
	ReplicationControllerFieldFsgid                         = "fsgid"
	ReplicationControllerFieldGids                          = "gids"
	ReplicationControllerFieldHostAliases                   = "hostAliases"
	ReplicationControllerFieldHostname                      = "hostname"
	ReplicationControllerFieldIPC                           = "ipc"
	ReplicationControllerFieldLabels                        = "labels"
	ReplicationControllerFieldName                          = "name"
	ReplicationControllerFieldNamespaceId                   = "namespaceId"
	ReplicationControllerFieldNet                           = "net"
	ReplicationControllerFieldNodeId                        = "nodeId"
	ReplicationControllerFieldOwnerReferences               = "ownerReferences"
	ReplicationControllerFieldPID                           = "pid"
	ReplicationControllerFieldPriority                      = "priority"
	ReplicationControllerFieldPriorityClassName             = "priorityClassName"
	ReplicationControllerFieldProjectID                     = "projectId"
	ReplicationControllerFieldPullSecrets                   = "pullSecrets"
	ReplicationControllerFieldRemoved                       = "removed"
	ReplicationControllerFieldRestart                       = "restart"
	ReplicationControllerFieldRunAsNonRoot                  = "runAsNonRoot"
	ReplicationControllerFieldScale                         = "scale"
	ReplicationControllerFieldSchedulerName                 = "schedulerName"
	ReplicationControllerFieldScheduling                    = "scheduling"
	ReplicationControllerFieldServiceAccountName            = "serviceAccountName"
	ReplicationControllerFieldState                         = "state"
	ReplicationControllerFieldStatus                        = "status"
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
	ActiveDeadlineSeconds         *int64                       `json:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string            `json:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                        `json:"automountServiceAccountToken,omitempty"`
	BatchSize                     string                       `json:"batchSize,omitempty"`
	Containers                    []Container                  `json:"containers,omitempty"`
	Created                       string                       `json:"created,omitempty"`
	DNSPolicy                     string                       `json:"dnsPolicy,omitempty"`
	DeploymentStrategy            *DeployStrategy              `json:"deploymentStrategy,omitempty"`
	Finalizers                    []string                     `json:"finalizers,omitempty"`
	Fsgid                         *int64                       `json:"fsgid,omitempty"`
	Gids                          []int64                      `json:"gids,omitempty"`
	HostAliases                   map[string]HostAlias         `json:"hostAliases,omitempty"`
	Hostname                      string                       `json:"hostname,omitempty"`
	IPC                           string                       `json:"ipc,omitempty"`
	Labels                        map[string]string            `json:"labels,omitempty"`
	Name                          string                       `json:"name,omitempty"`
	NamespaceId                   string                       `json:"namespaceId,omitempty"`
	Net                           string                       `json:"net,omitempty"`
	NodeId                        string                       `json:"nodeId,omitempty"`
	OwnerReferences               []OwnerReference             `json:"ownerReferences,omitempty"`
	PID                           string                       `json:"pid,omitempty"`
	Priority                      *int64                       `json:"priority,omitempty"`
	PriorityClassName             string                       `json:"priorityClassName,omitempty"`
	ProjectID                     string                       `json:"projectId,omitempty"`
	PullSecrets                   []LocalObjectReference       `json:"pullSecrets,omitempty"`
	Removed                       string                       `json:"removed,omitempty"`
	Restart                       string                       `json:"restart,omitempty"`
	RunAsNonRoot                  *bool                        `json:"runAsNonRoot,omitempty"`
	Scale                         *int64                       `json:"scale,omitempty"`
	SchedulerName                 string                       `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling                  `json:"scheduling,omitempty"`
	ServiceAccountName            string                       `json:"serviceAccountName,omitempty"`
	State                         string                       `json:"state,omitempty"`
	Status                        *ReplicationControllerStatus `json:"status,omitempty"`
	Subdomain                     string                       `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                       `json:"terminationGracePeriodSeconds,omitempty"`
	Transitioning                 string                       `json:"transitioning,omitempty"`
	TransitioningMessage          string                       `json:"transitioningMessage,omitempty"`
	Uid                           *int64                       `json:"uid,omitempty"`
	Uuid                          string                       `json:"uuid,omitempty"`
	Volumes                       map[string]Volume            `json:"volumes,omitempty"`
	WorkloadAnnotations           map[string]string            `json:"workloadAnnotations,omitempty"`
	WorkloadLabels                map[string]string            `json:"workloadLabels,omitempty"`
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
