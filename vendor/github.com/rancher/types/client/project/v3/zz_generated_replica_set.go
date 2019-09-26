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
	ReplicaSetFieldDNSConfig                     = "dnsConfig"
	ReplicaSetFieldDNSPolicy                     = "dnsPolicy"
	ReplicaSetFieldEnableServiceLinks            = "enableServiceLinks"
	ReplicaSetFieldEphemeralContainers           = "ephemeralContainers"
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
	ReplicaSetFieldNodeID                        = "nodeId"
	ReplicaSetFieldOverhead                      = "overhead"
	ReplicaSetFieldOwnerReferences               = "ownerReferences"
	ReplicaSetFieldPreemptionPolicy              = "preemptionPolicy"
	ReplicaSetFieldProjectID                     = "projectId"
	ReplicaSetFieldPublicEndpoints               = "publicEndpoints"
	ReplicaSetFieldReadinessGates                = "readinessGates"
	ReplicaSetFieldRemoved                       = "removed"
	ReplicaSetFieldReplicaSetConfig              = "replicaSetConfig"
	ReplicaSetFieldReplicaSetStatus              = "replicaSetStatus"
	ReplicaSetFieldRestartPolicy                 = "restartPolicy"
	ReplicaSetFieldRunAsGroup                    = "runAsGroup"
	ReplicaSetFieldRunAsNonRoot                  = "runAsNonRoot"
	ReplicaSetFieldRuntimeClassName              = "runtimeClassName"
	ReplicaSetFieldScale                         = "scale"
	ReplicaSetFieldScheduling                    = "scheduling"
	ReplicaSetFieldSelector                      = "selector"
	ReplicaSetFieldServiceAccountName            = "serviceAccountName"
	ReplicaSetFieldShareProcessNamespace         = "shareProcessNamespace"
	ReplicaSetFieldState                         = "state"
	ReplicaSetFieldSubdomain                     = "subdomain"
	ReplicaSetFieldSysctls                       = "sysctls"
	ReplicaSetFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	ReplicaSetFieldTopologySpreadConstraints     = "topologySpreadConstraints"
	ReplicaSetFieldTransitioning                 = "transitioning"
	ReplicaSetFieldTransitioningMessage          = "transitioningMessage"
	ReplicaSetFieldUUID                          = "uuid"
	ReplicaSetFieldUid                           = "uid"
	ReplicaSetFieldVolumes                       = "volumes"
	ReplicaSetFieldWindowsOptions                = "windowsOptions"
	ReplicaSetFieldWorkloadAnnotations           = "workloadAnnotations"
	ReplicaSetFieldWorkloadLabels                = "workloadLabels"
	ReplicaSetFieldWorkloadMetrics               = "workloadMetrics"
)

type ReplicaSet struct {
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
	ReplicaSetConfig              *ReplicaSetConfig              `json:"replicaSetConfig,omitempty" yaml:"replicaSetConfig,omitempty"`
	ReplicaSetStatus              *ReplicaSetStatus              `json:"replicaSetStatus,omitempty" yaml:"replicaSetStatus,omitempty"`
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
	Replace(existing *ReplicaSet) (*ReplicaSet, error)
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

func (c *ReplicaSetClient) Replace(obj *ReplicaSet) (*ReplicaSet, error) {
	resp := &ReplicaSet{}
	err := c.apiClient.Ops.DoReplace(ReplicaSetType, &obj.Resource, obj, resp)
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
