package client

import (
	"github.com/rancher/norman/types"
)

const (
	DeploymentType                               = "deployment"
	DeploymentFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	DeploymentFieldAnnotations                   = "annotations"
	DeploymentFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	DeploymentFieldContainers                    = "containers"
	DeploymentFieldCreated                       = "created"
	DeploymentFieldCreatorID                     = "creatorId"
	DeploymentFieldDNSPolicy                     = "dnsPolicy"
	DeploymentFieldDeployment                    = "deployment"
	DeploymentFieldDeploymentStatus              = "deploymentStatus"
	DeploymentFieldFsgid                         = "fsgid"
	DeploymentFieldGids                          = "gids"
	DeploymentFieldHostAliases                   = "hostAliases"
	DeploymentFieldHostIPC                       = "hostIPC"
	DeploymentFieldHostNetwork                   = "hostNetwork"
	DeploymentFieldHostPID                       = "hostPID"
	DeploymentFieldHostname                      = "hostname"
	DeploymentFieldImagePullSecrets              = "imagePullSecrets"
	DeploymentFieldLabels                        = "labels"
	DeploymentFieldName                          = "name"
	DeploymentFieldNamespaceId                   = "namespaceId"
	DeploymentFieldNodeId                        = "nodeId"
	DeploymentFieldOwnerReferences               = "ownerReferences"
	DeploymentFieldPriority                      = "priority"
	DeploymentFieldPriorityClassName             = "priorityClassName"
	DeploymentFieldProjectID                     = "projectId"
	DeploymentFieldRemoved                       = "removed"
	DeploymentFieldRestartPolicy                 = "restartPolicy"
	DeploymentFieldRunAsNonRoot                  = "runAsNonRoot"
	DeploymentFieldScale                         = "scale"
	DeploymentFieldSchedulerName                 = "schedulerName"
	DeploymentFieldScheduling                    = "scheduling"
	DeploymentFieldSelector                      = "selector"
	DeploymentFieldServiceAccountName            = "serviceAccountName"
	DeploymentFieldState                         = "state"
	DeploymentFieldSubdomain                     = "subdomain"
	DeploymentFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	DeploymentFieldTransitioning                 = "transitioning"
	DeploymentFieldTransitioningMessage          = "transitioningMessage"
	DeploymentFieldUid                           = "uid"
	DeploymentFieldUuid                          = "uuid"
	DeploymentFieldVolumes                       = "volumes"
	DeploymentFieldWorkloadAnnotations           = "workloadAnnotations"
	DeploymentFieldWorkloadLabels                = "workloadLabels"
)

type Deployment struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string      `json:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty"`
	Created                       string                 `json:"created,omitempty"`
	CreatorID                     string                 `json:"creatorId,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	Deployment                    *DeploymentConfig      `json:"deployment,omitempty"`
	DeploymentStatus              *DeploymentStatus      `json:"deploymentStatus,omitempty"`
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
	Scale                         *int64                 `json:"scale,omitempty"`
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
type DeploymentCollection struct {
	types.Collection
	Data   []Deployment `json:"data,omitempty"`
	client *DeploymentClient
}

type DeploymentClient struct {
	apiClient *Client
}

type DeploymentOperations interface {
	List(opts *types.ListOpts) (*DeploymentCollection, error)
	Create(opts *Deployment) (*Deployment, error)
	Update(existing *Deployment, updates interface{}) (*Deployment, error)
	ByID(id string) (*Deployment, error)
	Delete(container *Deployment) error
}

func newDeploymentClient(apiClient *Client) *DeploymentClient {
	return &DeploymentClient{
		apiClient: apiClient,
	}
}

func (c *DeploymentClient) Create(container *Deployment) (*Deployment, error) {
	resp := &Deployment{}
	err := c.apiClient.Ops.DoCreate(DeploymentType, container, resp)
	return resp, err
}

func (c *DeploymentClient) Update(existing *Deployment, updates interface{}) (*Deployment, error) {
	resp := &Deployment{}
	err := c.apiClient.Ops.DoUpdate(DeploymentType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *DeploymentClient) List(opts *types.ListOpts) (*DeploymentCollection, error) {
	resp := &DeploymentCollection{}
	err := c.apiClient.Ops.DoList(DeploymentType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *DeploymentCollection) Next() (*DeploymentCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &DeploymentCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *DeploymentClient) ByID(id string) (*Deployment, error) {
	resp := &Deployment{}
	err := c.apiClient.Ops.DoByID(DeploymentType, id, resp)
	return resp, err
}

func (c *DeploymentClient) Delete(container *Deployment) error {
	return c.apiClient.Ops.DoResourceDelete(DeploymentType, &container.Resource)
}
