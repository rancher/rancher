package client

import (
	"github.com/rancher/norman/types"
)

const (
	ReplicaSetType                               = "replicaSet"
	ReplicaSetFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	ReplicaSetFieldAnnotations                   = "annotations"
	ReplicaSetFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	ReplicaSetFieldContainers                    = "containers"
	ReplicaSetFieldCreated                       = "created"
	ReplicaSetFieldCreatorID                     = "creatorId"
	ReplicaSetFieldDNSPolicy                     = "dnsPolicy"
	ReplicaSetFieldFsgid                         = "fsgid"
	ReplicaSetFieldGids                          = "gids"
	ReplicaSetFieldHostAliases                   = "hostAliases"
	ReplicaSetFieldHostIPC                       = "hostIPC"
	ReplicaSetFieldHostNetwork                   = "hostNetwork"
	ReplicaSetFieldHostPID                       = "hostPID"
	ReplicaSetFieldHostname                      = "hostname"
	ReplicaSetFieldImagePullSecrets              = "imagePullSecrets"
	ReplicaSetFieldLabels                        = "labels"
	ReplicaSetFieldName                          = "name"
	ReplicaSetFieldNamespaceId                   = "namespaceId"
	ReplicaSetFieldNodeId                        = "nodeId"
	ReplicaSetFieldOwnerReferences               = "ownerReferences"
	ReplicaSetFieldPriority                      = "priority"
	ReplicaSetFieldPriorityClassName             = "priorityClassName"
	ReplicaSetFieldProjectID                     = "projectId"
	ReplicaSetFieldRemoved                       = "removed"
	ReplicaSetFieldReplicaSet                    = "replicaSet"
	ReplicaSetFieldReplicaSetStatus              = "replicaSetStatus"
	ReplicaSetFieldRestartPolicy                 = "restartPolicy"
	ReplicaSetFieldRunAsNonRoot                  = "runAsNonRoot"
	ReplicaSetFieldSchedulerName                 = "schedulerName"
	ReplicaSetFieldScheduling                    = "scheduling"
	ReplicaSetFieldSelector                      = "selector"
	ReplicaSetFieldServiceAccountName            = "serviceAccountName"
	ReplicaSetFieldState                         = "state"
	ReplicaSetFieldSubdomain                     = "subdomain"
	ReplicaSetFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	ReplicaSetFieldTransitioning                 = "transitioning"
	ReplicaSetFieldTransitioningMessage          = "transitioningMessage"
	ReplicaSetFieldUid                           = "uid"
	ReplicaSetFieldUuid                          = "uuid"
	ReplicaSetFieldVolumes                       = "volumes"
	ReplicaSetFieldWorkloadAnnotations           = "workloadAnnotations"
	ReplicaSetFieldWorkloadLabels                = "workloadLabels"
)

type ReplicaSet struct {
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
	OwnerReferences               []OwnerReference       `json:"ownerReferences,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty"`
	ProjectID                     string                 `json:"projectId,omitempty"`
	Removed                       string                 `json:"removed,omitempty"`
	ReplicaSet                    *ReplicaSetConfig      `json:"replicaSet,omitempty"`
	ReplicaSetStatus              *ReplicaSetStatus      `json:"replicaSetStatus,omitempty"`
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
type ReplicaSetCollection struct {
	types.Collection
	Data   []ReplicaSet `json:"data,omitempty"`
	client *ReplicaSetClient
}

type ReplicaSetClient struct {
	apiClient *Client
}

type ReplicaSetOperations interface {
	List(opts *types.ListOpts) (*ReplicaSetCollection, error)
	Create(opts *ReplicaSet) (*ReplicaSet, error)
	Update(existing *ReplicaSet, updates interface{}) (*ReplicaSet, error)
	ByID(id string) (*ReplicaSet, error)
	Delete(container *ReplicaSet) error
}

func newReplicaSetClient(apiClient *Client) *ReplicaSetClient {
	return &ReplicaSetClient{
		apiClient: apiClient,
	}
}

func (c *ReplicaSetClient) Create(container *ReplicaSet) (*ReplicaSet, error) {
	resp := &ReplicaSet{}
	err := c.apiClient.Ops.DoCreate(ReplicaSetType, container, resp)
	return resp, err
}

func (c *ReplicaSetClient) Update(existing *ReplicaSet, updates interface{}) (*ReplicaSet, error) {
	resp := &ReplicaSet{}
	err := c.apiClient.Ops.DoUpdate(ReplicaSetType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ReplicaSetClient) List(opts *types.ListOpts) (*ReplicaSetCollection, error) {
	resp := &ReplicaSetCollection{}
	err := c.apiClient.Ops.DoList(ReplicaSetType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ReplicaSetCollection) Next() (*ReplicaSetCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ReplicaSetCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ReplicaSetClient) ByID(id string) (*ReplicaSet, error) {
	resp := &ReplicaSet{}
	err := c.apiClient.Ops.DoByID(ReplicaSetType, id, resp)
	return resp, err
}

func (c *ReplicaSetClient) Delete(container *ReplicaSet) error {
	return c.apiClient.Ops.DoResourceDelete(ReplicaSetType, &container.Resource)
}
