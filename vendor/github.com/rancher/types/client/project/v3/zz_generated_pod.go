package client

import (
	"github.com/rancher/norman/types"
)

const (
	PodType                               = "pod"
	PodFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	PodFieldAnnotations                   = "annotations"
	PodFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	PodFieldContainers                    = "containers"
	PodFieldCreated                       = "created"
	PodFieldDNSPolicy                     = "dnsPolicy"
	PodFieldFinalizers                    = "finalizers"
	PodFieldFsgid                         = "fsgid"
	PodFieldGids                          = "gids"
	PodFieldHostAliases                   = "hostAliases"
	PodFieldHostname                      = "hostname"
	PodFieldIPC                           = "ipc"
	PodFieldLabels                        = "labels"
	PodFieldName                          = "name"
	PodFieldNamespaceId                   = "namespaceId"
	PodFieldNet                           = "net"
	PodFieldNodeId                        = "nodeId"
	PodFieldOwnerReferences               = "ownerReferences"
	PodFieldPID                           = "pid"
	PodFieldPriority                      = "priority"
	PodFieldPriorityClassName             = "priorityClassName"
	PodFieldProjectID                     = "projectId"
	PodFieldPullSecrets                   = "pullSecrets"
	PodFieldRemoved                       = "removed"
	PodFieldResourcePath                  = "resourcePath"
	PodFieldRestart                       = "restart"
	PodFieldRunAsNonRoot                  = "runAsNonRoot"
	PodFieldSchedulerName                 = "schedulerName"
	PodFieldScheduling                    = "scheduling"
	PodFieldServiceAccountName            = "serviceAccountName"
	PodFieldState                         = "state"
	PodFieldStatus                        = "status"
	PodFieldSubdomain                     = "subdomain"
	PodFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	PodFieldTolerations                   = "tolerations"
	PodFieldTransitioning                 = "transitioning"
	PodFieldTransitioningMessage          = "transitioningMessage"
	PodFieldUid                           = "uid"
	PodFieldUuid                          = "uuid"
	PodFieldVolumes                       = "volumes"
	PodFieldWorkloadID                    = "workloadId"
)

type Pod struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string      `json:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty"`
	Created                       string                 `json:"created,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	Finalizers                    []string               `json:"finalizers,omitempty"`
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
	PullSecrets                   []LocalObjectReference `json:"pullSecrets,omitempty"`
	Removed                       string                 `json:"removed,omitempty"`
	ResourcePath                  string                 `json:"resourcePath,omitempty"`
	Restart                       string                 `json:"restart,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty"`
	State                         string                 `json:"state,omitempty"`
	Status                        *PodStatus             `json:"status,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Tolerations                   []Toleration           `json:"tolerations,omitempty"`
	Transitioning                 string                 `json:"transitioning,omitempty"`
	TransitioningMessage          string                 `json:"transitioningMessage,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Uuid                          string                 `json:"uuid,omitempty"`
	Volumes                       map[string]Volume      `json:"volumes,omitempty"`
	WorkloadID                    string                 `json:"workloadId,omitempty"`
}
type PodCollection struct {
	types.Collection
	Data   []Pod `json:"data,omitempty"`
	client *PodClient
}

type PodClient struct {
	apiClient *Client
}

type PodOperations interface {
	List(opts *types.ListOpts) (*PodCollection, error)
	Create(opts *Pod) (*Pod, error)
	Update(existing *Pod, updates interface{}) (*Pod, error)
	ByID(id string) (*Pod, error)
	Delete(container *Pod) error
}

func newPodClient(apiClient *Client) *PodClient {
	return &PodClient{
		apiClient: apiClient,
	}
}

func (c *PodClient) Create(container *Pod) (*Pod, error) {
	resp := &Pod{}
	err := c.apiClient.Ops.DoCreate(PodType, container, resp)
	return resp, err
}

func (c *PodClient) Update(existing *Pod, updates interface{}) (*Pod, error) {
	resp := &Pod{}
	err := c.apiClient.Ops.DoUpdate(PodType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *PodClient) List(opts *types.ListOpts) (*PodCollection, error) {
	resp := &PodCollection{}
	err := c.apiClient.Ops.DoList(PodType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *PodCollection) Next() (*PodCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &PodCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *PodClient) ByID(id string) (*Pod, error) {
	resp := &Pod{}
	err := c.apiClient.Ops.DoByID(PodType, id, resp)
	return resp, err
}

func (c *PodClient) Delete(container *Pod) error {
	return c.apiClient.Ops.DoResourceDelete(PodType, &container.Resource)
}
