package client

import (
	"github.com/rancher/norman/types"
)

const (
	CronJobType                               = "cronJob"
	CronJobFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	CronJobFieldAnnotations                   = "annotations"
	CronJobFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	CronJobFieldContainers                    = "containers"
	CronJobFieldCreated                       = "created"
	CronJobFieldCreatorID                     = "creatorId"
	CronJobFieldCronJob                       = "cronJob"
	CronJobFieldCronJobStatus                 = "cronJobStatus"
	CronJobFieldDNSPolicy                     = "dnsPolicy"
	CronJobFieldFsgid                         = "fsgid"
	CronJobFieldGids                          = "gids"
	CronJobFieldHostAliases                   = "hostAliases"
	CronJobFieldHostIPC                       = "hostIPC"
	CronJobFieldHostNetwork                   = "hostNetwork"
	CronJobFieldHostPID                       = "hostPID"
	CronJobFieldHostname                      = "hostname"
	CronJobFieldImagePullSecrets              = "imagePullSecrets"
	CronJobFieldLabels                        = "labels"
	CronJobFieldName                          = "name"
	CronJobFieldNamespaceId                   = "namespaceId"
	CronJobFieldNodeId                        = "nodeId"
	CronJobFieldOwnerReferences               = "ownerReferences"
	CronJobFieldPriority                      = "priority"
	CronJobFieldPriorityClassName             = "priorityClassName"
	CronJobFieldProjectID                     = "projectId"
	CronJobFieldPublicEndpoints               = "publicEndpoints"
	CronJobFieldRemoved                       = "removed"
	CronJobFieldRestartPolicy                 = "restartPolicy"
	CronJobFieldRunAsNonRoot                  = "runAsNonRoot"
	CronJobFieldSchedulerName                 = "schedulerName"
	CronJobFieldScheduling                    = "scheduling"
	CronJobFieldSelector                      = "selector"
	CronJobFieldServiceAccountName            = "serviceAccountName"
	CronJobFieldState                         = "state"
	CronJobFieldSubdomain                     = "subdomain"
	CronJobFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	CronJobFieldTransitioning                 = "transitioning"
	CronJobFieldTransitioningMessage          = "transitioningMessage"
	CronJobFieldUid                           = "uid"
	CronJobFieldUuid                          = "uuid"
	CronJobFieldVolumes                       = "volumes"
	CronJobFieldWorkloadAnnotations           = "workloadAnnotations"
	CronJobFieldWorkloadLabels                = "workloadLabels"
)

type CronJob struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string      `json:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty"`
	Created                       string                 `json:"created,omitempty"`
	CreatorID                     string                 `json:"creatorId,omitempty"`
	CronJob                       *CronJobConfig         `json:"cronJob,omitempty"`
	CronJobStatus                 *CronJobStatus         `json:"cronJobStatus,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	Fsgid                         *int64                 `json:"fsgid,omitempty"`
	Gids                          []int64                `json:"gids,omitempty"`
	HostAliases                   []HostAlias            `json:"hostAliases,omitempty"`
	HostIPC                       bool                   `json:"hostIPC,omitempty"`
	HostNetwork                   bool                   `json:"hostNetwork,omitempty"`
	HostPID                       bool                   `json:"hostPID,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference `json:"imagePullSecrets,omitempty"`
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
type CronJobCollection struct {
	types.Collection
	Data   []CronJob `json:"data,omitempty"`
	client *CronJobClient
}

type CronJobClient struct {
	apiClient *Client
}

type CronJobOperations interface {
	List(opts *types.ListOpts) (*CronJobCollection, error)
	Create(opts *CronJob) (*CronJob, error)
	Update(existing *CronJob, updates interface{}) (*CronJob, error)
	ByID(id string) (*CronJob, error)
	Delete(container *CronJob) error
}

func newCronJobClient(apiClient *Client) *CronJobClient {
	return &CronJobClient{
		apiClient: apiClient,
	}
}

func (c *CronJobClient) Create(container *CronJob) (*CronJob, error) {
	resp := &CronJob{}
	err := c.apiClient.Ops.DoCreate(CronJobType, container, resp)
	return resp, err
}

func (c *CronJobClient) Update(existing *CronJob, updates interface{}) (*CronJob, error) {
	resp := &CronJob{}
	err := c.apiClient.Ops.DoUpdate(CronJobType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *CronJobClient) List(opts *types.ListOpts) (*CronJobCollection, error) {
	resp := &CronJobCollection{}
	err := c.apiClient.Ops.DoList(CronJobType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *CronJobCollection) Next() (*CronJobCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &CronJobCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *CronJobClient) ByID(id string) (*CronJob, error) {
	resp := &CronJob{}
	err := c.apiClient.Ops.DoByID(CronJobType, id, resp)
	return resp, err
}

func (c *CronJobClient) Delete(container *CronJob) error {
	return c.apiClient.Ops.DoResourceDelete(CronJobType, &container.Resource)
}
