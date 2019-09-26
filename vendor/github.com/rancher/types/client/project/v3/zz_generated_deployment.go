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
	DeploymentFieldDNSConfig                     = "dnsConfig"
	DeploymentFieldDNSPolicy                     = "dnsPolicy"
	DeploymentFieldDeploymentConfig              = "deploymentConfig"
	DeploymentFieldDeploymentStatus              = "deploymentStatus"
	DeploymentFieldEnableServiceLinks            = "enableServiceLinks"
	DeploymentFieldEphemeralContainers           = "ephemeralContainers"
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
	DeploymentFieldNodeID                        = "nodeId"
	DeploymentFieldOverhead                      = "overhead"
	DeploymentFieldOwnerReferences               = "ownerReferences"
	DeploymentFieldPaused                        = "paused"
	DeploymentFieldPreemptionPolicy              = "preemptionPolicy"
	DeploymentFieldProjectID                     = "projectId"
	DeploymentFieldPublicEndpoints               = "publicEndpoints"
	DeploymentFieldReadinessGates                = "readinessGates"
	DeploymentFieldRemoved                       = "removed"
	DeploymentFieldRestartPolicy                 = "restartPolicy"
	DeploymentFieldRunAsGroup                    = "runAsGroup"
	DeploymentFieldRunAsNonRoot                  = "runAsNonRoot"
	DeploymentFieldRuntimeClassName              = "runtimeClassName"
	DeploymentFieldScale                         = "scale"
	DeploymentFieldScheduling                    = "scheduling"
	DeploymentFieldSelector                      = "selector"
	DeploymentFieldServiceAccountName            = "serviceAccountName"
	DeploymentFieldShareProcessNamespace         = "shareProcessNamespace"
	DeploymentFieldState                         = "state"
	DeploymentFieldSubdomain                     = "subdomain"
	DeploymentFieldSysctls                       = "sysctls"
	DeploymentFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	DeploymentFieldTopologySpreadConstraints     = "topologySpreadConstraints"
	DeploymentFieldTransitioning                 = "transitioning"
	DeploymentFieldTransitioningMessage          = "transitioningMessage"
	DeploymentFieldUUID                          = "uuid"
	DeploymentFieldUid                           = "uid"
	DeploymentFieldVolumes                       = "volumes"
	DeploymentFieldWindowsOptions                = "windowsOptions"
	DeploymentFieldWorkloadAnnotations           = "workloadAnnotations"
	DeploymentFieldWorkloadLabels                = "workloadLabels"
	DeploymentFieldWorkloadMetrics               = "workloadMetrics"
)

type Deployment struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                         `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string              `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                          `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	Containers                    []Container                    `json:"containers,omitempty" yaml:"containers,omitempty"`
	Created                       string                         `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                     string                         `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DNSConfig                     *PodDNSConfig                  `json:"dnsConfig,omitempty" yaml:"dnsConfig,omitempty"`
	DNSPolicy                     string                         `json:"dnsPolicy,omitempty" yaml:"dnsPolicy,omitempty"`
	DeploymentConfig              *DeploymentConfig              `json:"deploymentConfig,omitempty" yaml:"deploymentConfig,omitempty"`
	DeploymentStatus              *DeploymentStatus              `json:"deploymentStatus,omitempty" yaml:"deploymentStatus,omitempty"`
	EnableServiceLinks            *bool                          `json:"enableServiceLinks,omitempty" yaml:"enableServiceLinks,omitempty"`
	EphemeralContainers           []EphemeralContainer           `json:"ephemeralContainers,omitempty" yaml:"ephemeralContainers,omitempty"`
	Fsgid                         *int64                         `json:"fsgid,omitempty" yaml:"fsgid,omitempty"`
	Gids                          []int64                        `json:"gids,omitempty" yaml:"gids,omitempty"`
	HostAliases                   []HostAlias                    `json:"hostAliases,omitempty" yaml:"hostAliases,omitempty"`
	HostIPC                       bool                           `json:"hostIPC,omitempty" yaml:"hostIPC,omitempty"`
	HostNetwork                   bool                           `json:"hostNetwork,omitempty" yaml:"hostNetwork,omitempty"`
	HostPID                       bool                           `json:"hostPID,omitempty" yaml:"hostPID,omitempty"`
	Hostname                      string                         `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference         `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	Labels                        map[string]string              `json:"labels,omitempty" yaml:"labels,omitempty"`
	Name                          string                         `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId                   string                         `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NodeID                        string                         `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	Overhead                      map[string]string              `json:"overhead,omitempty" yaml:"overhead,omitempty"`
	OwnerReferences               []OwnerReference               `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	Paused                        bool                           `json:"paused,omitempty" yaml:"paused,omitempty"`
	PreemptionPolicy              string                         `json:"preemptionPolicy,omitempty" yaml:"preemptionPolicy,omitempty"`
	ProjectID                     string                         `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	PublicEndpoints               []PublicEndpoint               `json:"publicEndpoints,omitempty" yaml:"publicEndpoints,omitempty"`
	ReadinessGates                []PodReadinessGate             `json:"readinessGates,omitempty" yaml:"readinessGates,omitempty"`
	Removed                       string                         `json:"removed,omitempty" yaml:"removed,omitempty"`
	RestartPolicy                 string                         `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsGroup                    *int64                         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot                  *bool                          `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	RuntimeClassName              string                         `json:"runtimeClassName,omitempty" yaml:"runtimeClassName,omitempty"`
	Scale                         *int64                         `json:"scale,omitempty" yaml:"scale,omitempty"`
	Scheduling                    *Scheduling                    `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	Selector                      *LabelSelector                 `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceAccountName            string                         `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	ShareProcessNamespace         *bool                          `json:"shareProcessNamespace,omitempty" yaml:"shareProcessNamespace,omitempty"`
	State                         string                         `json:"state,omitempty" yaml:"state,omitempty"`
	Subdomain                     string                         `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	Sysctls                       []Sysctl                       `json:"sysctls,omitempty" yaml:"sysctls,omitempty"`
	TerminationGracePeriodSeconds *int64                         `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	TopologySpreadConstraints     []TopologySpreadConstraint     `json:"topologySpreadConstraints,omitempty" yaml:"topologySpreadConstraints,omitempty"`
	Transitioning                 string                         `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage          string                         `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	UUID                          string                         `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Uid                           *int64                         `json:"uid,omitempty" yaml:"uid,omitempty"`
	Volumes                       []Volume                       `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WindowsOptions                *WindowsSecurityContextOptions `json:"windowsOptions,omitempty" yaml:"windowsOptions,omitempty"`
	WorkloadAnnotations           map[string]string              `json:"workloadAnnotations,omitempty" yaml:"workloadAnnotations,omitempty"`
	WorkloadLabels                map[string]string              `json:"workloadLabels,omitempty" yaml:"workloadLabels,omitempty"`
	WorkloadMetrics               []WorkloadMetric               `json:"workloadMetrics,omitempty" yaml:"workloadMetrics,omitempty"`
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
	Replace(existing *Deployment) (*Deployment, error)
	ByID(id string) (*Deployment, error)
	Delete(container *Deployment) error

	ActionPause(resource *Deployment) error

	ActionResume(resource *Deployment) error

	ActionRollback(resource *Deployment, input *DeploymentRollbackInput) error
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

func (c *DeploymentClient) Replace(obj *Deployment) (*Deployment, error) {
	resp := &Deployment{}
	err := c.apiClient.Ops.DoReplace(DeploymentType, &obj.Resource, obj, resp)
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

func (c *DeploymentClient) ActionPause(resource *Deployment) error {
	err := c.apiClient.Ops.DoAction(DeploymentType, "pause", &resource.Resource, nil, nil)
	return err
}

func (c *DeploymentClient) ActionResume(resource *Deployment) error {
	err := c.apiClient.Ops.DoAction(DeploymentType, "resume", &resource.Resource, nil, nil)
	return err
}

func (c *DeploymentClient) ActionRollback(resource *Deployment, input *DeploymentRollbackInput) error {
	err := c.apiClient.Ops.DoAction(DeploymentType, "rollback", &resource.Resource, input, nil)
	return err
}
