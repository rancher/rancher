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
	StatefulSetFieldObjectMeta                    = "metadata"
	StatefulSetFieldOwnerReferences               = "ownerReferences"
	StatefulSetFieldPriority                      = "priority"
	StatefulSetFieldPriorityClassName             = "priorityClassName"
	StatefulSetFieldProjectID                     = "projectId"
	StatefulSetFieldRemoved                       = "removed"
	StatefulSetFieldRestartPolicy                 = "restartPolicy"
	StatefulSetFieldRunAsNonRoot                  = "runAsNonRoot"
	StatefulSetFieldSchedulerName                 = "schedulerName"
	StatefulSetFieldScheduling                    = "scheduling"
	StatefulSetFieldSelector                      = "selector"
	StatefulSetFieldServiceAccountName            = "serviceAccountName"
	StatefulSetFieldState                         = "state"
	StatefulSetFieldStatefulSet                   = "statefulSet"
	StatefulSetFieldStatefulSetStatus             = "statefulSetStatus"
	StatefulSetFieldSubdomain                     = "subdomain"
	StatefulSetFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	StatefulSetFieldTransitioning                 = "transitioning"
	StatefulSetFieldTransitioningMessage          = "transitioningMessage"
	StatefulSetFieldUid                           = "uid"
	StatefulSetFieldUuid                          = "uuid"
	StatefulSetFieldVolumes                       = "volumes"
)

type StatefulSet struct {
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
	HostIPC                       *bool                  `json:"hostIPC,omitempty"`
	HostNetwork                   *bool                  `json:"hostNetwork,omitempty"`
	HostPID                       *bool                  `json:"hostPID,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference `json:"imagePullSecrets,omitempty"`
	Labels                        map[string]string      `json:"labels,omitempty"`
	Name                          string                 `json:"name,omitempty"`
	NamespaceId                   string                 `json:"namespaceId,omitempty"`
	NodeId                        string                 `json:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta            `json:"metadata,omitempty"`
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
	StatefulSet                   *StatefulSetConfig     `json:"statefulSet,omitempty"`
	StatefulSetStatus             *StatefulSetStatus     `json:"statefulSetStatus,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Transitioning                 string                 `json:"transitioning,omitempty"`
	TransitioningMessage          string                 `json:"transitioningMessage,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Uuid                          string                 `json:"uuid,omitempty"`
	Volumes                       []Volume               `json:"volumes,omitempty"`
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
