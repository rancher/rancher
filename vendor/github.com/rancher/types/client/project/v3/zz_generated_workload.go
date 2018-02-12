package client

import (
	"github.com/rancher/norman/types"
)

const (
	WorkloadType                               = "workload"
	WorkloadFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	WorkloadFieldAnnotations                   = "annotations"
	WorkloadFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	WorkloadFieldContainers                    = "containers"
	WorkloadFieldCreated                       = "created"
	WorkloadFieldCreatorID                     = "creatorId"
	WorkloadFieldCronJob                       = "cronJob"
	WorkloadFieldCronJobStatus                 = "cronJobStatus"
	WorkloadFieldDNSPolicy                     = "dnsPolicy"
	WorkloadFieldDaemonSet                     = "daemonSet"
	WorkloadFieldDaemonSetStatus               = "daemonSetStatus"
	WorkloadFieldDeployment                    = "deployment"
	WorkloadFieldDeploymentStatus              = "deploymentStatus"
	WorkloadFieldFsgid                         = "fsgid"
	WorkloadFieldGids                          = "gids"
	WorkloadFieldHostAliases                   = "hostAliases"
	WorkloadFieldHostIPC                       = "hostIPC"
	WorkloadFieldHostNetwork                   = "hostNetwork"
	WorkloadFieldHostPID                       = "hostPID"
	WorkloadFieldHostname                      = "hostname"
	WorkloadFieldImagePullSecrets              = "imagePullSecrets"
	WorkloadFieldJob                           = "job"
	WorkloadFieldJobStatus                     = "jobStatus"
	WorkloadFieldLabels                        = "labels"
	WorkloadFieldName                          = "name"
	WorkloadFieldNamespaceId                   = "namespaceId"
	WorkloadFieldNodeId                        = "nodeId"
	WorkloadFieldOwnerReferences               = "ownerReferences"
	WorkloadFieldPriority                      = "priority"
	WorkloadFieldPriorityClassName             = "priorityClassName"
	WorkloadFieldProjectID                     = "projectId"
	WorkloadFieldRemoved                       = "removed"
	WorkloadFieldReplicationController         = "replicationController"
	WorkloadFieldReplicationControllerStatus   = "replicationControllerStatus"
	WorkloadFieldRestartPolicy                 = "restartPolicy"
	WorkloadFieldRunAsNonRoot                  = "runAsNonRoot"
	WorkloadFieldScale                         = "scale"
	WorkloadFieldSchedulerName                 = "schedulerName"
	WorkloadFieldScheduling                    = "scheduling"
	WorkloadFieldSelector                      = "selector"
	WorkloadFieldServiceAccountName            = "serviceAccountName"
	WorkloadFieldState                         = "state"
	WorkloadFieldStatefulSet                   = "statefulSet"
	WorkloadFieldStatefulSetStatus             = "statefulSetStatus"
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
	ActiveDeadlineSeconds         *int64                       `json:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string            `json:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                        `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container                  `json:"containers,omitempty"`
	Created                       string                       `json:"created,omitempty"`
	CreatorID                     string                       `json:"creatorId,omitempty"`
	CronJob                       *CronJobConfig               `json:"cronJob,omitempty"`
	CronJobStatus                 *CronJobStatus               `json:"cronJobStatus,omitempty"`
	DNSPolicy                     string                       `json:"dnsPolicy,omitempty"`
	DaemonSet                     *DaemonSetConfig             `json:"daemonSet,omitempty"`
	DaemonSetStatus               *DaemonSetStatus             `json:"daemonSetStatus,omitempty"`
	Deployment                    *DeploymentConfig            `json:"deployment,omitempty"`
	DeploymentStatus              *DeploymentStatus            `json:"deploymentStatus,omitempty"`
	Fsgid                         *int64                       `json:"fsgid,omitempty"`
	Gids                          []int64                      `json:"gids,omitempty"`
	HostAliases                   []HostAlias                  `json:"hostAliases,omitempty"`
	HostIPC                       *bool                        `json:"hostIPC,omitempty"`
	HostNetwork                   *bool                        `json:"hostNetwork,omitempty"`
	HostPID                       *bool                        `json:"hostPID,omitempty"`
	Hostname                      string                       `json:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference       `json:"imagePullSecrets,omitempty"`
	Job                           *JobConfig                   `json:"job,omitempty"`
	JobStatus                     *JobStatus                   `json:"jobStatus,omitempty"`
	Labels                        map[string]string            `json:"labels,omitempty"`
	Name                          string                       `json:"name,omitempty"`
	NamespaceId                   string                       `json:"namespaceId,omitempty"`
	NodeId                        string                       `json:"nodeId,omitempty"`
	OwnerReferences               []OwnerReference             `json:"ownerReferences,omitempty"`
	Priority                      *int64                       `json:"priority,omitempty"`
	PriorityClassName             string                       `json:"priorityClassName,omitempty"`
	ProjectID                     string                       `json:"projectId,omitempty"`
	Removed                       string                       `json:"removed,omitempty"`
	ReplicationController         *ReplicationControllerConfig `json:"replicationController,omitempty"`
	ReplicationControllerStatus   *ReplicationControllerStatus `json:"replicationControllerStatus,omitempty"`
	RestartPolicy                 string                       `json:"restartPolicy,omitempty"`
	RunAsNonRoot                  *bool                        `json:"runAsNonRoot,omitempty"`
	Scale                         *int64                       `json:"scale,omitempty"`
	SchedulerName                 string                       `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling                  `json:"scheduling,omitempty"`
	Selector                      *LabelSelector               `json:"selector,omitempty"`
	ServiceAccountName            string                       `json:"serviceAccountName,omitempty"`
	State                         string                       `json:"state,omitempty"`
	StatefulSet                   *StatefulSetConfig           `json:"statefulSet,omitempty"`
	StatefulSetStatus             *StatefulSetStatus           `json:"statefulSetStatus,omitempty"`
	Subdomain                     string                       `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                       `json:"terminationGracePeriodSeconds,omitempty"`
	Transitioning                 string                       `json:"transitioning,omitempty"`
	TransitioningMessage          string                       `json:"transitioningMessage,omitempty"`
	Uid                           *int64                       `json:"uid,omitempty"`
	Uuid                          string                       `json:"uuid,omitempty"`
	Volumes                       []Volume                     `json:"volumes,omitempty"`
	WorkloadAnnotations           map[string]string            `json:"workloadAnnotations,omitempty"`
	WorkloadLabels                map[string]string            `json:"workloadLabels,omitempty"`
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
