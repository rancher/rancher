package client

import (
	"github.com/rancher/norman/types"
)

const (
	StatefulSetType                               = "statefulSet"
	StatefulSetFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	StatefulSetFieldAnnotations                   = "annotations"
	StatefulSetFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	StatefulSetFieldContainers                    = "containers"
	StatefulSetFieldCreated                       = "created"
	StatefulSetFieldCreatorID                     = "creatorId"
	StatefulSetFieldDNSConfig                     = "dnsConfig"
	StatefulSetFieldDNSPolicy                     = "dnsPolicy"
	StatefulSetFieldFsgid                         = "fsgid"
	StatefulSetFieldGids                          = "gids"
	StatefulSetFieldHostAliases                   = "hostAliases"
	StatefulSetFieldHostIPC                       = "hostIPC"
	StatefulSetFieldHostNetwork                   = "hostNetwork"
	StatefulSetFieldHostPID                       = "hostPID"
	StatefulSetFieldHostname                      = "hostname"
	StatefulSetFieldImagePullSecrets              = "imagePullSecrets"
	StatefulSetFieldLabels                        = "labels"
	StatefulSetFieldName                          = "name"
	StatefulSetFieldNamespaceId                   = "namespaceId"
	StatefulSetFieldNodeId                        = "nodeId"
	StatefulSetFieldOwnerReferences               = "ownerReferences"
	StatefulSetFieldPriority                      = "priority"
	StatefulSetFieldPriorityClassName             = "priorityClassName"
	StatefulSetFieldProjectID                     = "projectId"
	StatefulSetFieldPublicEndpoints               = "publicEndpoints"
	StatefulSetFieldRemoved                       = "removed"
	StatefulSetFieldRestartPolicy                 = "restartPolicy"
	StatefulSetFieldRunAsGroup                    = "runAsGroup"
	StatefulSetFieldRunAsNonRoot                  = "runAsNonRoot"
	StatefulSetFieldScale                         = "scale"
	StatefulSetFieldSchedulerName                 = "schedulerName"
	StatefulSetFieldScheduling                    = "scheduling"
	StatefulSetFieldSelector                      = "selector"
	StatefulSetFieldServiceAccountName            = "serviceAccountName"
	StatefulSetFieldShareProcessNamespace         = "shareProcessNamespace"
	StatefulSetFieldState                         = "state"
	StatefulSetFieldStatefulSetConfig             = "statefulSetConfig"
	StatefulSetFieldStatefulSetStatus             = "statefulSetStatus"
	StatefulSetFieldSubdomain                     = "subdomain"
	StatefulSetFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	StatefulSetFieldTransitioning                 = "transitioning"
	StatefulSetFieldTransitioningMessage          = "transitioningMessage"
	StatefulSetFieldUid                           = "uid"
	StatefulSetFieldUuid                          = "uuid"
	StatefulSetFieldVolumes                       = "volumes"
	StatefulSetFieldWorkloadAnnotations           = "workloadAnnotations"
	StatefulSetFieldWorkloadLabels                = "workloadLabels"
)

type StatefulSet struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string      `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty" yaml:"containers,omitempty"`
	Created                       string                 `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                     string                 `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DNSConfig                     *PodDNSConfig          `json:"dnsConfig,omitempty" yaml:"dnsConfig,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty" yaml:"dnsPolicy,omitempty"`
	Fsgid                         *int64                 `json:"fsgid,omitempty" yaml:"fsgid,omitempty"`
	Gids                          []int64                `json:"gids,omitempty" yaml:"gids,omitempty"`
	HostAliases                   []HostAlias            `json:"hostAliases,omitempty" yaml:"hostAliases,omitempty"`
	HostIPC                       bool                   `json:"hostIPC,omitempty" yaml:"hostIPC,omitempty"`
	HostNetwork                   bool                   `json:"hostNetwork,omitempty" yaml:"hostNetwork,omitempty"`
	HostPID                       bool                   `json:"hostPID,omitempty" yaml:"hostPID,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	Labels                        map[string]string      `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                          string                 `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId                   string                 `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NodeId                        string                 `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	OwnerReferences               []OwnerReference       `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty" yaml:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	ProjectID                     string                 `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	PublicEndpoints               []PublicEndpoint       `json:"publicEndpoints,omitempty" yaml:"publicEndpoints,omitempty"`
	Removed                       string                 `json:"removed,omitempty" yaml:"removed,omitempty"`
	RestartPolicy                 string                 `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsGroup                    *int64                 `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	Scale                         *int64                 `json:"scale,omitempty" yaml:"scale,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty" yaml:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	Selector                      *LabelSelector         `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	ShareProcessNamespace         *bool                  `json:"shareProcessNamespace,omitempty" yaml:"shareProcessNamespace,omitempty"`
	State                         string                 `json:"state,omitempty" yaml:"state,omitempty"`
	StatefulSetConfig             *StatefulSetConfig     `json:"statefulSetConfig,omitempty" yaml:"statefulSetConfig,omitempty"`
	StatefulSetStatus             *StatefulSetStatus     `json:"statefulSetStatus,omitempty" yaml:"statefulSetStatus,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	Transitioning                 string                 `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage          string                 `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty" yaml:"uid,omitempty"`
	Uuid                          string                 `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Volumes                       []Volume               `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WorkloadAnnotations           map[string]string      `json:"workloadAnnotations,omitempty" yaml:"workloadAnnotations,omitempty"`
	WorkloadLabels                map[string]string      `json:"workloadLabels,omitempty" yaml:"workloadLabels,omitempty"`
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
