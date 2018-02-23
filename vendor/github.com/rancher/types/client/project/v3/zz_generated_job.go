package client

import (
	"github.com/rancher/norman/types"
)

const (
	JobType                               = "job"
	JobFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	JobFieldAnnotations                   = "annotations"
	JobFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	JobFieldContainers                    = "containers"
	JobFieldCreated                       = "created"
	JobFieldCreatorID                     = "creatorId"
	JobFieldDNSPolicy                     = "dnsPolicy"
	JobFieldFsgid                         = "fsgid"
	JobFieldGids                          = "gids"
	JobFieldHostAliases                   = "hostAliases"
	JobFieldHostIPC                       = "hostIPC"
	JobFieldHostNetwork                   = "hostNetwork"
	JobFieldHostPID                       = "hostPID"
	JobFieldHostname                      = "hostname"
	JobFieldImagePullSecrets              = "imagePullSecrets"
	JobFieldJob                           = "job"
	JobFieldJobStatus                     = "jobStatus"
	JobFieldLabels                        = "labels"
	JobFieldName                          = "name"
	JobFieldNamespaceId                   = "namespaceId"
	JobFieldNodeId                        = "nodeId"
	JobFieldOwnerReferences               = "ownerReferences"
	JobFieldPriority                      = "priority"
	JobFieldPriorityClassName             = "priorityClassName"
	JobFieldProjectID                     = "projectId"
	JobFieldPublicEndpoints               = "publicEndpoints"
	JobFieldRemoved                       = "removed"
	JobFieldRestartPolicy                 = "restartPolicy"
	JobFieldRunAsNonRoot                  = "runAsNonRoot"
	JobFieldSchedulerName                 = "schedulerName"
	JobFieldScheduling                    = "scheduling"
	JobFieldSelector                      = "selector"
	JobFieldServiceAccountName            = "serviceAccountName"
	JobFieldState                         = "state"
	JobFieldSubdomain                     = "subdomain"
	JobFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	JobFieldTransitioning                 = "transitioning"
	JobFieldTransitioningMessage          = "transitioningMessage"
	JobFieldUid                           = "uid"
	JobFieldUuid                          = "uuid"
	JobFieldVolumes                       = "volumes"
	JobFieldWorkloadAnnotations           = "workloadAnnotations"
	JobFieldWorkloadLabels                = "workloadLabels"
)

type Job struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string      `json:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty"`
	Created                       string                 `json:"created,omitempty"`
	CreatorID                     string                 `json:"creatorId,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	Fsgid                         *int64                 `json:"fsgid,omitempty"`
	Gids                          []int64                `json:"gids,omitempty"`
	HostAliases                   []HostAlias            `json:"hostAliases,omitempty"`
	HostIPC                       bool                   `json:"hostIPC,omitempty"`
	HostNetwork                   bool                   `json:"hostNetwork,omitempty"`
	HostPID                       bool                   `json:"hostPID,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference `json:"imagePullSecrets,omitempty"`
	Job                           *JobConfig             `json:"job,omitempty"`
	JobStatus                     *JobStatus             `json:"jobStatus,omitempty"`
	Labels                        map[string]string      `json:"labels,omitempty"`
	Name                          string                 `json:"name,omitempty"`
	NamespaceId                   string                 `json:"namespaceId,omitempty"`
	NodeId                        string                 `json:"nodeId,omitempty"`
	OwnerReferences               []OwnerReference       `json:"ownerReferences,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty"`
	ProjectID                     string                 `json:"projectId,omitempty"`
	PublicEndpoints               []PublicEndpoint       `json:"publicEndpoints,omitempty"`
	Removed                       string                 `json:"removed,omitempty"`
	RestartPolicy                 string                 `json:"restartPolicy,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty"`
	Selector                      *LabelSelector         `json:"selector,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty"`
	State                         string                 `json:"state,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Transitioning                 string                 `json:"transitioning,omitempty"`
	TransitioningMessage          string                 `json:"transitioningMessage,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Uuid                          string                 `json:"uuid,omitempty"`
	Volumes                       []Volume               `json:"volumes,omitempty"`
	WorkloadAnnotations           map[string]string      `json:"workloadAnnotations,omitempty"`
	WorkloadLabels                map[string]string      `json:"workloadLabels,omitempty"`
}
type JobCollection struct {
	types.Collection
	Data   []Job `json:"data,omitempty"`
	client *JobClient
}

type JobClient struct {
	apiClient *Client
}

type JobOperations interface {
	List(opts *types.ListOpts) (*JobCollection, error)
	Create(opts *Job) (*Job, error)
	Update(existing *Job, updates interface{}) (*Job, error)
	ByID(id string) (*Job, error)
	Delete(container *Job) error
}

func newJobClient(apiClient *Client) *JobClient {
	return &JobClient{
		apiClient: apiClient,
	}
}

func (c *JobClient) Create(container *Job) (*Job, error) {
	resp := &Job{}
	err := c.apiClient.Ops.DoCreate(JobType, container, resp)
	return resp, err
}

func (c *JobClient) Update(existing *Job, updates interface{}) (*Job, error) {
	resp := &Job{}
	err := c.apiClient.Ops.DoUpdate(JobType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *JobClient) List(opts *types.ListOpts) (*JobCollection, error) {
	resp := &JobCollection{}
	err := c.apiClient.Ops.DoList(JobType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *JobCollection) Next() (*JobCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &JobCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *JobClient) ByID(id string) (*Job, error) {
	resp := &Job{}
	err := c.apiClient.Ops.DoByID(JobType, id, resp)
	return resp, err
}

func (c *JobClient) Delete(container *Job) error {
	return c.apiClient.Ops.DoResourceDelete(JobType, &container.Resource)
}
