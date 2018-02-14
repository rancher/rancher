package client

import (
	"github.com/rancher/norman/types"
)

const (
	DaemonSetType                               = "daemonSet"
	DaemonSetFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	DaemonSetFieldAnnotations                   = "annotations"
	DaemonSetFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	DaemonSetFieldContainers                    = "containers"
	DaemonSetFieldCreated                       = "created"
	DaemonSetFieldCreatorID                     = "creatorId"
	DaemonSetFieldDNSPolicy                     = "dnsPolicy"
	DaemonSetFieldDaemonSet                     = "daemonSet"
	DaemonSetFieldDaemonSetStatus               = "daemonSetStatus"
	DaemonSetFieldFsgid                         = "fsgid"
	DaemonSetFieldGids                          = "gids"
	DaemonSetFieldHostAliases                   = "hostAliases"
	DaemonSetFieldHostIPC                       = "hostIPC"
	DaemonSetFieldHostNetwork                   = "hostNetwork"
	DaemonSetFieldHostPID                       = "hostPID"
	DaemonSetFieldHostname                      = "hostname"
	DaemonSetFieldImagePullSecrets              = "imagePullSecrets"
	DaemonSetFieldLabels                        = "labels"
	DaemonSetFieldName                          = "name"
	DaemonSetFieldNamespaceId                   = "namespaceId"
	DaemonSetFieldNodeId                        = "nodeId"
	DaemonSetFieldOwnerReferences               = "ownerReferences"
	DaemonSetFieldPriority                      = "priority"
	DaemonSetFieldPriorityClassName             = "priorityClassName"
	DaemonSetFieldProjectID                     = "projectId"
	DaemonSetFieldRemoved                       = "removed"
	DaemonSetFieldRestartPolicy                 = "restartPolicy"
	DaemonSetFieldRunAsNonRoot                  = "runAsNonRoot"
	DaemonSetFieldSchedulerName                 = "schedulerName"
	DaemonSetFieldScheduling                    = "scheduling"
	DaemonSetFieldSelector                      = "selector"
	DaemonSetFieldServiceAccountName            = "serviceAccountName"
	DaemonSetFieldState                         = "state"
	DaemonSetFieldSubdomain                     = "subdomain"
	DaemonSetFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	DaemonSetFieldTransitioning                 = "transitioning"
	DaemonSetFieldTransitioningMessage          = "transitioningMessage"
	DaemonSetFieldUid                           = "uid"
	DaemonSetFieldUuid                          = "uuid"
	DaemonSetFieldVolumes                       = "volumes"
	DaemonSetFieldWorkloadAnnotations           = "workloadAnnotations"
	DaemonSetFieldWorkloadLabels                = "workloadLabels"
)

type DaemonSet struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string      `json:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty"`
	Created                       string                 `json:"created,omitempty"`
	CreatorID                     string                 `json:"creatorId,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	DaemonSet                     *DaemonSetConfig       `json:"daemonSet,omitempty"`
	DaemonSetStatus               *DaemonSetStatus       `json:"daemonSetStatus,omitempty"`
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
