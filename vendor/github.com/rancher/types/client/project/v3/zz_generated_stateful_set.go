package client

import (
	"github.com/rancher/norman/types"
)

const (
	StatefulSetType                               = "statefulSet"
	StatefulSetField                              = "creatorId"
	StatefulSetFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	StatefulSetFieldAnnotations                   = "annotations"
	StatefulSetFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	StatefulSetFieldBatchSize                     = "batchSize"
	StatefulSetFieldContainers                    = "containers"
	StatefulSetFieldCreated                       = "created"
	StatefulSetFieldDNSPolicy                     = "dnsPolicy"
	StatefulSetFieldDeploymentStrategy            = "deploymentStrategy"
	StatefulSetFieldFinalizers                    = "finalizers"
	StatefulSetFieldFsgid                         = "fsgid"
	StatefulSetFieldGids                          = "gids"
	StatefulSetFieldHostAliases                   = "hostAliases"
	StatefulSetFieldHostname                      = "hostname"
	StatefulSetFieldIPC                           = "ipc"
	StatefulSetFieldLabels                        = "labels"
	StatefulSetFieldName                          = "name"
	StatefulSetFieldNamespaceId                   = "namespaceId"
	StatefulSetFieldNet                           = "net"
	StatefulSetFieldNodeId                        = "nodeId"
	StatefulSetFieldOwnerReferences               = "ownerReferences"
	StatefulSetFieldPID                           = "pid"
	StatefulSetFieldPodManagementPolicy           = "podManagementPolicy"
	StatefulSetFieldPriority                      = "priority"
	StatefulSetFieldPriorityClassName             = "priorityClassName"
	StatefulSetFieldProjectID                     = "projectId"
	StatefulSetFieldPullSecrets                   = "pullSecrets"
	StatefulSetFieldRemoved                       = "removed"
	StatefulSetFieldRestart                       = "restart"
	StatefulSetFieldRevisionHistoryLimit          = "revisionHistoryLimit"
	StatefulSetFieldRunAsNonRoot                  = "runAsNonRoot"
	StatefulSetFieldScale                         = "scale"
	StatefulSetFieldSchedulerName                 = "schedulerName"
	StatefulSetFieldScheduling                    = "scheduling"
	StatefulSetFieldServiceAccountName            = "serviceAccountName"
	StatefulSetFieldServiceName                   = "serviceName"
	StatefulSetFieldState                         = "state"
	StatefulSetFieldStatus                        = "status"
	StatefulSetFieldSubdomain                     = "subdomain"
	StatefulSetFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	StatefulSetFieldTransitioning                 = "transitioning"
	StatefulSetFieldTransitioningMessage          = "transitioningMessage"
	StatefulSetFieldUid                           = "uid"
	StatefulSetFieldUpdateStrategy                = "updateStrategy"
	StatefulSetFieldUuid                          = "uuid"
	StatefulSetFieldVolumeClaimTemplates          = "volumeClaimTemplates"
	StatefulSetFieldVolumes                       = "volumes"
	StatefulSetFieldWorkloadAnnotations           = "workloadAnnotations"
	StatefulSetFieldWorkloadLabels                = "workloadLabels"
)

type StatefulSet struct {
	types.Resource
	string                        `json:"creatorId,omitempty"`
	ActiveDeadlineSeconds         *int64                     `json:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string          `json:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                      `json:"automountServiceAccountToken,omitempty"`
	BatchSize                     string                     `json:"batchSize,omitempty"`
	Containers                    []Container                `json:"containers,omitempty"`
	Created                       string                     `json:"created,omitempty"`
	DNSPolicy                     string                     `json:"dnsPolicy,omitempty"`
	DeploymentStrategy            *DeployStrategy            `json:"deploymentStrategy,omitempty"`
	Finalizers                    []string                   `json:"finalizers,omitempty"`
	Fsgid                         *int64                     `json:"fsgid,omitempty"`
	Gids                          []int64                    `json:"gids,omitempty"`
	HostAliases                   map[string]HostAlias       `json:"hostAliases,omitempty"`
	Hostname                      string                     `json:"hostname,omitempty"`
	IPC                           string                     `json:"ipc,omitempty"`
	Labels                        map[string]string          `json:"labels,omitempty"`
	Name                          string                     `json:"name,omitempty"`
	NamespaceId                   string                     `json:"namespaceId,omitempty"`
	Net                           string                     `json:"net,omitempty"`
	NodeId                        string                     `json:"nodeId,omitempty"`
	OwnerReferences               []OwnerReference           `json:"ownerReferences,omitempty"`
	PID                           string                     `json:"pid,omitempty"`
	PodManagementPolicy           string                     `json:"podManagementPolicy,omitempty"`
	Priority                      *int64                     `json:"priority,omitempty"`
	PriorityClassName             string                     `json:"priorityClassName,omitempty"`
	ProjectID                     string                     `json:"projectId,omitempty"`
	PullSecrets                   []LocalObjectReference     `json:"pullSecrets,omitempty"`
	Removed                       string                     `json:"removed,omitempty"`
	Restart                       string                     `json:"restart,omitempty"`
	RevisionHistoryLimit          *int64                     `json:"revisionHistoryLimit,omitempty"`
	RunAsNonRoot                  *bool                      `json:"runAsNonRoot,omitempty"`
	Scale                         *int64                     `json:"scale,omitempty"`
	SchedulerName                 string                     `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling                `json:"scheduling,omitempty"`
	ServiceAccountName            string                     `json:"serviceAccountName,omitempty"`
	ServiceName                   string                     `json:"serviceName,omitempty"`
	State                         string                     `json:"state,omitempty"`
	Status                        *StatefulSetStatus         `json:"status,omitempty"`
	Subdomain                     string                     `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                     `json:"terminationGracePeriodSeconds,omitempty"`
	Transitioning                 string                     `json:"transitioning,omitempty"`
	TransitioningMessage          string                     `json:"transitioningMessage,omitempty"`
	Uid                           *int64                     `json:"uid,omitempty"`
	UpdateStrategy                *StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`
	Uuid                          string                     `json:"uuid,omitempty"`
	VolumeClaimTemplates          []PersistentVolumeClaim    `json:"volumeClaimTemplates,omitempty"`
	Volumes                       map[string]Volume          `json:"volumes,omitempty"`
	WorkloadAnnotations           map[string]string          `json:"workloadAnnotations,omitempty"`
	WorkloadLabels                map[string]string          `json:"workloadLabels,omitempty"`
}
type StatefulSetCollection struct {
	types.Collection
	Data   []StatefulSet `json:"data,omitempty"`
	client *StatefulSetClient
}

type StatefulSetClient struct {
	apiClient *Client
}

type StatefulSetOperations interface {
	List(opts *types.ListOpts) (*StatefulSetCollection, error)
	Create(opts *StatefulSet) (*StatefulSet, error)
	Update(existing *StatefulSet, updates interface{}) (*StatefulSet, error)
	ByID(id string) (*StatefulSet, error)
	Delete(container *StatefulSet) error
}

func newStatefulSetClient(apiClient *Client) *StatefulSetClient {
	return &StatefulSetClient{
		apiClient: apiClient,
	}
}

func (c *StatefulSetClient) Create(container *StatefulSet) (*StatefulSet, error) {
	resp := &StatefulSet{}
	err := c.apiClient.Ops.DoCreate(StatefulSetType, container, resp)
	return resp, err
}

func (c *StatefulSetClient) Update(existing *StatefulSet, updates interface{}) (*StatefulSet, error) {
	resp := &StatefulSet{}
	err := c.apiClient.Ops.DoUpdate(StatefulSetType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *StatefulSetClient) List(opts *types.ListOpts) (*StatefulSetCollection, error) {
	resp := &StatefulSetCollection{}
	err := c.apiClient.Ops.DoList(StatefulSetType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *StatefulSetCollection) Next() (*StatefulSetCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &StatefulSetCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *StatefulSetClient) ByID(id string) (*StatefulSet, error) {
	resp := &StatefulSet{}
	err := c.apiClient.Ops.DoByID(StatefulSetType, id, resp)
	return resp, err
}

func (c *StatefulSetClient) Delete(container *StatefulSet) error {
	return c.apiClient.Ops.DoResourceDelete(StatefulSetType, &container.Resource)
}
