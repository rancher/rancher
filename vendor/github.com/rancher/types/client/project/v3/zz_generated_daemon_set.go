package client

import (
	"github.com/rancher/norman/types"
)

const (
	DaemonSetType                               = "daemonSet"
	DaemonSetField                              = "creatorId"
	DaemonSetFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	DaemonSetFieldAnnotations                   = "annotations"
	DaemonSetFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	DaemonSetFieldBatchSize                     = "batchSize"
	DaemonSetFieldContainers                    = "containers"
	DaemonSetFieldCreated                       = "created"
	DaemonSetFieldDNSPolicy                     = "dnsPolicy"
	DaemonSetFieldDeploymentStrategy            = "deploymentStrategy"
	DaemonSetFieldFinalizers                    = "finalizers"
	DaemonSetFieldFsgid                         = "fsgid"
	DaemonSetFieldGids                          = "gids"
	DaemonSetFieldHostAliases                   = "hostAliases"
	DaemonSetFieldHostname                      = "hostname"
	DaemonSetFieldIPC                           = "ipc"
	DaemonSetFieldLabels                        = "labels"
	DaemonSetFieldName                          = "name"
	DaemonSetFieldNamespaceId                   = "namespaceId"
	DaemonSetFieldNet                           = "net"
	DaemonSetFieldNodeId                        = "nodeId"
	DaemonSetFieldOwnerReferences               = "ownerReferences"
	DaemonSetFieldPID                           = "pid"
	DaemonSetFieldPriority                      = "priority"
	DaemonSetFieldPriorityClassName             = "priorityClassName"
	DaemonSetFieldProjectID                     = "projectId"
	DaemonSetFieldPullSecrets                   = "pullSecrets"
	DaemonSetFieldRemoved                       = "removed"
	DaemonSetFieldRestart                       = "restart"
	DaemonSetFieldRevisionHistoryLimit          = "revisionHistoryLimit"
	DaemonSetFieldRunAsNonRoot                  = "runAsNonRoot"
	DaemonSetFieldScale                         = "scale"
	DaemonSetFieldSchedulerName                 = "schedulerName"
	DaemonSetFieldScheduling                    = "scheduling"
	DaemonSetFieldServiceAccountName            = "serviceAccountName"
	DaemonSetFieldState                         = "state"
	DaemonSetFieldStatus                        = "status"
	DaemonSetFieldSubdomain                     = "subdomain"
	DaemonSetFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	DaemonSetFieldTransitioning                 = "transitioning"
	DaemonSetFieldTransitioningMessage          = "transitioningMessage"
	DaemonSetFieldUid                           = "uid"
	DaemonSetFieldUpdateStrategy                = "updateStrategy"
	DaemonSetFieldUuid                          = "uuid"
	DaemonSetFieldVolumes                       = "volumes"
	DaemonSetFieldWorkloadAnnotations           = "workloadAnnotations"
	DaemonSetFieldWorkloadLabels                = "workloadLabels"
)

type DaemonSet struct {
	types.Resource
	string                        `json:"creatorId,omitempty"`
	ActiveDeadlineSeconds         *int64                   `json:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string        `json:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                    `json:"automountServiceAccountToken,omitempty"`
	BatchSize                     string                   `json:"batchSize,omitempty"`
	Containers                    []Container              `json:"containers,omitempty"`
	Created                       string                   `json:"created,omitempty"`
	DNSPolicy                     string                   `json:"dnsPolicy,omitempty"`
	DeploymentStrategy            *DeployStrategy          `json:"deploymentStrategy,omitempty"`
	Finalizers                    []string                 `json:"finalizers,omitempty"`
	Fsgid                         *int64                   `json:"fsgid,omitempty"`
	Gids                          []int64                  `json:"gids,omitempty"`
	HostAliases                   map[string]HostAlias     `json:"hostAliases,omitempty"`
	Hostname                      string                   `json:"hostname,omitempty"`
	IPC                           string                   `json:"ipc,omitempty"`
	Labels                        map[string]string        `json:"labels,omitempty"`
	Name                          string                   `json:"name,omitempty"`
	NamespaceId                   string                   `json:"namespaceId,omitempty"`
	Net                           string                   `json:"net,omitempty"`
	NodeId                        string                   `json:"nodeId,omitempty"`
	OwnerReferences               []OwnerReference         `json:"ownerReferences,omitempty"`
	PID                           string                   `json:"pid,omitempty"`
	Priority                      *int64                   `json:"priority,omitempty"`
	PriorityClassName             string                   `json:"priorityClassName,omitempty"`
	ProjectID                     string                   `json:"projectId,omitempty"`
	PullSecrets                   []LocalObjectReference   `json:"pullSecrets,omitempty"`
	Removed                       string                   `json:"removed,omitempty"`
	Restart                       string                   `json:"restart,omitempty"`
	RevisionHistoryLimit          *int64                   `json:"revisionHistoryLimit,omitempty"`
	RunAsNonRoot                  *bool                    `json:"runAsNonRoot,omitempty"`
	Scale                         *int64                   `json:"scale,omitempty"`
	SchedulerName                 string                   `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling              `json:"scheduling,omitempty"`
	ServiceAccountName            string                   `json:"serviceAccountName,omitempty"`
	State                         string                   `json:"state,omitempty"`
	Status                        *DaemonSetStatus         `json:"status,omitempty"`
	Subdomain                     string                   `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                   `json:"terminationGracePeriodSeconds,omitempty"`
	Transitioning                 string                   `json:"transitioning,omitempty"`
	TransitioningMessage          string                   `json:"transitioningMessage,omitempty"`
	Uid                           *int64                   `json:"uid,omitempty"`
	UpdateStrategy                *DaemonSetUpdateStrategy `json:"updateStrategy,omitempty"`
	Uuid                          string                   `json:"uuid,omitempty"`
	Volumes                       map[string]Volume        `json:"volumes,omitempty"`
	WorkloadAnnotations           map[string]string        `json:"workloadAnnotations,omitempty"`
	WorkloadLabels                map[string]string        `json:"workloadLabels,omitempty"`
}
type DaemonSetCollection struct {
	types.Collection
	Data   []DaemonSet `json:"data,omitempty"`
	client *DaemonSetClient
}

type DaemonSetClient struct {
	apiClient *Client
}

type DaemonSetOperations interface {
	List(opts *types.ListOpts) (*DaemonSetCollection, error)
	Create(opts *DaemonSet) (*DaemonSet, error)
	Update(existing *DaemonSet, updates interface{}) (*DaemonSet, error)
	ByID(id string) (*DaemonSet, error)
	Delete(container *DaemonSet) error
}

func newDaemonSetClient(apiClient *Client) *DaemonSetClient {
	return &DaemonSetClient{
		apiClient: apiClient,
	}
}

func (c *DaemonSetClient) Create(container *DaemonSet) (*DaemonSet, error) {
	resp := &DaemonSet{}
	err := c.apiClient.Ops.DoCreate(DaemonSetType, container, resp)
	return resp, err
}

func (c *DaemonSetClient) Update(existing *DaemonSet, updates interface{}) (*DaemonSet, error) {
	resp := &DaemonSet{}
	err := c.apiClient.Ops.DoUpdate(DaemonSetType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *DaemonSetClient) List(opts *types.ListOpts) (*DaemonSetCollection, error) {
	resp := &DaemonSetCollection{}
	err := c.apiClient.Ops.DoList(DaemonSetType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *DaemonSetCollection) Next() (*DaemonSetCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &DaemonSetCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *DaemonSetClient) ByID(id string) (*DaemonSet, error) {
	resp := &DaemonSet{}
	err := c.apiClient.Ops.DoByID(DaemonSetType, id, resp)
	return resp, err
}

func (c *DaemonSetClient) Delete(container *DaemonSet) error {
	return c.apiClient.Ops.DoResourceDelete(DaemonSetType, &container.Resource)
}
