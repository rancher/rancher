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
	ReplicationControllerFieldDNSConfig                     = "dnsConfig"
	ReplicationControllerFieldDNSPolicy                     = "dnsPolicy"
	ReplicationControllerFieldEnableServiceLinks            = "enableServiceLinks"
	ReplicationControllerFieldEphemeralContainers           = "ephemeralContainers"
	ReplicationControllerFieldFSGroupChangePolicy           = "fsGroupChangePolicy"
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
	ReplicationControllerFieldNodeID                        = "nodeId"
	ReplicationControllerFieldOverhead                      = "overhead"
	ReplicationControllerFieldOwnerReferences               = "ownerReferences"
	ReplicationControllerFieldPreemptionPolicy              = "preemptionPolicy"
	ReplicationControllerFieldProjectID                     = "projectId"
	ReplicationControllerFieldPublicEndpoints               = "publicEndpoints"
	ReplicationControllerFieldReadinessGates                = "readinessGates"
	ReplicationControllerFieldRemoved                       = "removed"
	ReplicationControllerFieldReplicationControllerConfig   = "replicationControllerConfig"
	ReplicationControllerFieldReplicationControllerStatus   = "replicationControllerStatus"
	ReplicationControllerFieldRestartPolicy                 = "restartPolicy"
	ReplicationControllerFieldRunAsGroup                    = "runAsGroup"
	ReplicationControllerFieldRunAsNonRoot                  = "runAsNonRoot"
	ReplicationControllerFieldRuntimeClassName              = "runtimeClassName"
	ReplicationControllerFieldScale                         = "scale"
	ReplicationControllerFieldScheduling                    = "scheduling"
	ReplicationControllerFieldSeccompProfile                = "seccompProfile"
	ReplicationControllerFieldSelector                      = "selector"
	ReplicationControllerFieldServiceAccountName            = "serviceAccountName"
	ReplicationControllerFieldSetHostnameAsFQDN             = "setHostnameAsFQDN"
	ReplicationControllerFieldShareProcessNamespace         = "shareProcessNamespace"
	ReplicationControllerFieldState                         = "state"
	ReplicationControllerFieldSubdomain                     = "subdomain"
	ReplicationControllerFieldSysctls                       = "sysctls"
	ReplicationControllerFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	ReplicationControllerFieldTopologySpreadConstraints     = "topologySpreadConstraints"
	ReplicationControllerFieldTransitioning                 = "transitioning"
	ReplicationControllerFieldTransitioningMessage          = "transitioningMessage"
	ReplicationControllerFieldUUID                          = "uuid"
	ReplicationControllerFieldUid                           = "uid"
	ReplicationControllerFieldVolumes                       = "volumes"
	ReplicationControllerFieldWindowsOptions                = "windowsOptions"
	ReplicationControllerFieldWorkloadAnnotations           = "workloadAnnotations"
	ReplicationControllerFieldWorkloadLabels                = "workloadLabels"
	ReplicationControllerFieldWorkloadMetrics               = "workloadMetrics"
)

type ReplicationController struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                         `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string              `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                          `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	Containers                    []Container                    `json:"containers,omitempty" yaml:"containers,omitempty"`
	Created                       string                         `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                     string                         `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DNSConfig                     *PodDNSConfig                  `json:"dnsConfig,omitempty" yaml:"dnsConfig,omitempty"`
	DNSPolicy                     string                         `json:"dnsPolicy,omitempty" yaml:"dnsPolicy,omitempty"`
	EnableServiceLinks            *bool                          `json:"enableServiceLinks,omitempty" yaml:"enableServiceLinks,omitempty"`
	EphemeralContainers           []EphemeralContainer           `json:"ephemeralContainers,omitempty" yaml:"ephemeralContainers,omitempty"`
	FSGroupChangePolicy           string                         `json:"fsGroupChangePolicy,omitempty" yaml:"fsGroupChangePolicy,omitempty"`
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
	PreemptionPolicy              string                         `json:"preemptionPolicy,omitempty" yaml:"preemptionPolicy,omitempty"`
	ProjectID                     string                         `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	PublicEndpoints               []PublicEndpoint               `json:"publicEndpoints,omitempty" yaml:"publicEndpoints,omitempty"`
	ReadinessGates                []PodReadinessGate             `json:"readinessGates,omitempty" yaml:"readinessGates,omitempty"`
	Removed                       string                         `json:"removed,omitempty" yaml:"removed,omitempty"`
	ReplicationControllerConfig   *ReplicationControllerConfig   `json:"replicationControllerConfig,omitempty" yaml:"replicationControllerConfig,omitempty"`
	ReplicationControllerStatus   *ReplicationControllerStatus   `json:"replicationControllerStatus,omitempty" yaml:"replicationControllerStatus,omitempty"`
	RestartPolicy                 string                         `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsGroup                    *int64                         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot                  *bool                          `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	RuntimeClassName              string                         `json:"runtimeClassName,omitempty" yaml:"runtimeClassName,omitempty"`
	Scale                         *int64                         `json:"scale,omitempty" yaml:"scale,omitempty"`
	Scheduling                    *Scheduling                    `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	SeccompProfile                *SeccompProfile                `json:"seccompProfile,omitempty" yaml:"seccompProfile,omitempty"`
	Selector                      map[string]string              `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceAccountName            string                         `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	SetHostnameAsFQDN             *bool                          `json:"setHostnameAsFQDN,omitempty" yaml:"setHostnameAsFQDN,omitempty"`
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
	ListAll(opts *types.ListOpts) (*ReplicationControllerCollection, error)
	Create(opts *ReplicationController) (*ReplicationController, error)
	Update(existing *ReplicationController, updates interface{}) (*ReplicationController, error)
	Replace(existing *ReplicationController) (*ReplicationController, error)
	ByID(id string) (*ReplicationController, error)
	Delete(container *ReplicationController) error

	ActionRedeploy(resource *ReplicationController) error
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

func (c *ReplicationControllerClient) Replace(obj *ReplicationController) (*ReplicationController, error) {
	resp := &ReplicationController{}
	err := c.apiClient.Ops.DoReplace(ReplicationControllerType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *ReplicationControllerClient) List(opts *types.ListOpts) (*ReplicationControllerCollection, error) {
	resp := &ReplicationControllerCollection{}
	err := c.apiClient.Ops.DoList(ReplicationControllerType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *ReplicationControllerClient) ListAll(opts *types.ListOpts) (*ReplicationControllerCollection, error) {
	resp := &ReplicationControllerCollection{}
	resp, err := c.List(opts)
	if err != nil {
		return resp, err
	}
	data := resp.Data
	for next, err := resp.Next(); next != nil && err == nil; next, err = next.Next() {
		data = append(data, next.Data...)
		resp = next
		resp.Data = data
	}
	if err != nil {
		return resp, err
	}
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

func (c *ReplicationControllerClient) ActionRedeploy(resource *ReplicationController) error {
	err := c.apiClient.Ops.DoAction(ReplicationControllerType, "redeploy", &resource.Resource, nil, nil)
	return err
}
