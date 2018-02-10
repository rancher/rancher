package client

import (
	"github.com/rancher/norman/types"
)

const (
	ReplicationControllerType                               = "replicationController"
	ReplicationControllerFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	ReplicationControllerFieldAnnotations                   = "annotations"
	ReplicationControllerFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	ReplicationControllerFieldContainers                    = "containers"
	ReplicationControllerFieldCreated                       = "created"
	ReplicationControllerFieldCreatorID                     = "creatorId"
	ReplicationControllerFieldDNSPolicy                     = "dnsPolicy"
	ReplicationControllerFieldFsgid                         = "fsgid"
	ReplicationControllerFieldGids                          = "gids"
	ReplicationControllerFieldHostAliases                   = "hostAliases"
	ReplicationControllerFieldHostIPC                       = "hostIPC"
	ReplicationControllerFieldHostNetwork                   = "hostNetwork"
	ReplicationControllerFieldHostPID                       = "hostPID"
	ReplicationControllerFieldHostname                      = "hostname"
	ReplicationControllerFieldImagePullSecrets              = "imagePullSecrets"
	ReplicationControllerFieldLabels                        = "labels"
	ReplicationControllerFieldName                          = "name"
	ReplicationControllerFieldNamespaceId                   = "namespaceId"
	ReplicationControllerFieldNodeId                        = "nodeId"
	ReplicationControllerFieldObjectMeta                    = "metadata"
	ReplicationControllerFieldOwnerReferences               = "ownerReferences"
	ReplicationControllerFieldPriority                      = "priority"
	ReplicationControllerFieldPriorityClassName             = "priorityClassName"
	ReplicationControllerFieldProjectID                     = "projectId"
	ReplicationControllerFieldRemoved                       = "removed"
	ReplicationControllerFieldReplicationController         = "replicationController"
	ReplicationControllerFieldReplicationControllerStatus   = "replicationControllerStatus"
	ReplicationControllerFieldRestartPolicy                 = "restartPolicy"
	ReplicationControllerFieldRunAsNonRoot                  = "runAsNonRoot"
	ReplicationControllerFieldSchedulerName                 = "schedulerName"
	ReplicationControllerFieldScheduling                    = "scheduling"
	ReplicationControllerFieldSelector                      = "selector"
	ReplicationControllerFieldServiceAccountName            = "serviceAccountName"
	ReplicationControllerFieldState                         = "state"
	ReplicationControllerFieldSubdomain                     = "subdomain"
	ReplicationControllerFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	ReplicationControllerFieldTransitioning                 = "transitioning"
	ReplicationControllerFieldTransitioningMessage          = "transitioningMessage"
	ReplicationControllerFieldUid                           = "uid"
	ReplicationControllerFieldUuid                          = "uuid"
	ReplicationControllerFieldVolumes                       = "volumes"
)

type ReplicationController struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                       `json:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string            `json:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                        `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container                  `json:"containers,omitempty"`
	Created                       string                       `json:"created,omitempty"`
	CreatorID                     string                       `json:"creatorId,omitempty"`
	DNSPolicy                     string                       `json:"dnsPolicy,omitempty"`
	Fsgid                         *int64                       `json:"fsgid,omitempty"`
	Gids                          []int64                      `json:"gids,omitempty"`
	HostAliases                   []HostAlias                  `json:"hostAliases,omitempty"`
	HostIPC                       *bool                        `json:"hostIPC,omitempty"`
	HostNetwork                   *bool                        `json:"hostNetwork,omitempty"`
	HostPID                       *bool                        `json:"hostPID,omitempty"`
	Hostname                      string                       `json:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference       `json:"imagePullSecrets,omitempty"`
	Labels                        map[string]string            `json:"labels,omitempty"`
	Name                          string                       `json:"name,omitempty"`
	NamespaceId                   string                       `json:"namespaceId,omitempty"`
	NodeId                        string                       `json:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta                  `json:"metadata,omitempty"`
	OwnerReferences               []OwnerReference             `json:"ownerReferences,omitempty"`
	Priority                      *int64                       `json:"priority,omitempty"`
	PriorityClassName             string                       `json:"priorityClassName,omitempty"`
	ProjectID                     string                       `json:"projectId,omitempty"`
	Removed                       string                       `json:"removed,omitempty"`
	ReplicationController         *ReplicationControllerConfig `json:"replicationController,omitempty"`
	ReplicationControllerStatus   *ReplicationControllerStatus `json:"replicationControllerStatus,omitempty"`
	RestartPolicy                 string                       `json:"restartPolicy,omitempty"`
	RunAsNonRoot                  *bool                        `json:"runAsNonRoot,omitempty"`
	SchedulerName                 string                       `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling                  `json:"scheduling,omitempty"`
	Selector                      map[string]string            `json:"selector,omitempty"`
	ServiceAccountName            string                       `json:"serviceAccountName,omitempty"`
	State                         string                       `json:"state,omitempty"`
	Subdomain                     string                       `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                       `json:"terminationGracePeriodSeconds,omitempty"`
	Transitioning                 string                       `json:"transitioning,omitempty"`
	TransitioningMessage          string                       `json:"transitioningMessage,omitempty"`
	Uid                           *int64                       `json:"uid,omitempty"`
	Uuid                          string                       `json:"uuid,omitempty"`
	Volumes                       []Volume                     `json:"volumes,omitempty"`
}
type ReplicationControllerCollection struct {
	types.Collection
	Data   []ReplicationController `json:"data,omitempty"`
	client *ReplicationControllerClient
}

type ReplicationControllerClient struct {
	apiClient *Client
}

type ReplicationControllerOperations interface {
	List(opts *types.ListOpts) (*ReplicationControllerCollection, error)
	Create(opts *ReplicationController) (*ReplicationController, error)
	Update(existing *ReplicationController, updates interface{}) (*ReplicationController, error)
	ByID(id string) (*ReplicationController, error)
	Delete(container *ReplicationController) error
}

func newReplicationControllerClient(apiClient *Client) *ReplicationControllerClient {
	return &ReplicationControllerClient{
		apiClient: apiClient,
	}
}

func (c *ReplicationControllerClient) Create(container *ReplicationController) (*ReplicationController, error) {
	resp := &ReplicationController{}
	err := c.apiClient.Ops.DoCreate(ReplicationControllerType, container, resp)
	return resp, err
}

func (c *ReplicationControllerClient) Update(existing *ReplicationController, updates interface{}) (*ReplicationController, error) {
	resp := &ReplicationController{}
	err := c.apiClient.Ops.DoUpdate(ReplicationControllerType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *ReplicationControllerClient) List(opts *types.ListOpts) (*ReplicationControllerCollection, error) {
	resp := &ReplicationControllerCollection{}
	err := c.apiClient.Ops.DoList(ReplicationControllerType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *ReplicationControllerCollection) Next() (*ReplicationControllerCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &ReplicationControllerCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *ReplicationControllerClient) ByID(id string) (*ReplicationController, error) {
	resp := &ReplicationController{}
	err := c.apiClient.Ops.DoByID(ReplicationControllerType, id, resp)
	return resp, err
}

func (c *ReplicationControllerClient) Delete(container *ReplicationController) error {
	return c.apiClient.Ops.DoResourceDelete(ReplicationControllerType, &container.Resource)
}
