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
	DaemonSetFieldDNSConfig                     = "dnsConfig"
	DaemonSetFieldDNSPolicy                     = "dnsPolicy"
	DaemonSetFieldDaemonSetConfig               = "daemonSetConfig"
	DaemonSetFieldDaemonSetStatus               = "daemonSetStatus"
	DaemonSetFieldEnableServiceLinks            = "enableServiceLinks"
	DaemonSetFieldEphemeralContainers           = "ephemeralContainers"
	DaemonSetFieldFSGroupChangePolicy           = "fsGroupChangePolicy"
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
	DaemonSetFieldNodeID                        = "nodeId"
	DaemonSetFieldOverhead                      = "overhead"
	DaemonSetFieldOwnerReferences               = "ownerReferences"
	DaemonSetFieldPreemptionPolicy              = "preemptionPolicy"
	DaemonSetFieldProjectID                     = "projectId"
	DaemonSetFieldPublicEndpoints               = "publicEndpoints"
	DaemonSetFieldReadinessGates                = "readinessGates"
	DaemonSetFieldRemoved                       = "removed"
	DaemonSetFieldRestartPolicy                 = "restartPolicy"
	DaemonSetFieldRunAsGroup                    = "runAsGroup"
	DaemonSetFieldRunAsNonRoot                  = "runAsNonRoot"
	DaemonSetFieldRuntimeClassName              = "runtimeClassName"
	DaemonSetFieldScheduling                    = "scheduling"
	DaemonSetFieldSelector                      = "selector"
	DaemonSetFieldServiceAccountName            = "serviceAccountName"
	DaemonSetFieldShareProcessNamespace         = "shareProcessNamespace"
	DaemonSetFieldState                         = "state"
	DaemonSetFieldSubdomain                     = "subdomain"
	DaemonSetFieldSysctls                       = "sysctls"
	DaemonSetFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	DaemonSetFieldTopologySpreadConstraints     = "topologySpreadConstraints"
	DaemonSetFieldTransitioning                 = "transitioning"
	DaemonSetFieldTransitioningMessage          = "transitioningMessage"
	DaemonSetFieldUUID                          = "uuid"
	DaemonSetFieldUid                           = "uid"
	DaemonSetFieldVolumes                       = "volumes"
	DaemonSetFieldWindowsOptions                = "windowsOptions"
	DaemonSetFieldWorkloadAnnotations           = "workloadAnnotations"
	DaemonSetFieldWorkloadLabels                = "workloadLabels"
	DaemonSetFieldWorkloadMetrics               = "workloadMetrics"
)

type DaemonSet struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                         `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string              `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                          `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	Containers                    []Container                    `json:"containers,omitempty" yaml:"containers,omitempty"`
	Created                       string                         `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                     string                         `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	DNSConfig                     *PodDNSConfig                  `json:"dnsConfig,omitempty" yaml:"dnsConfig,omitempty"`
	DNSPolicy                     string                         `json:"dnsPolicy,omitempty" yaml:"dnsPolicy,omitempty"`
	DaemonSetConfig               *DaemonSetConfig               `json:"daemonSetConfig,omitempty" yaml:"daemonSetConfig,omitempty"`
	DaemonSetStatus               *DaemonSetStatus               `json:"daemonSetStatus,omitempty" yaml:"daemonSetStatus,omitempty"`
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
	RestartPolicy                 string                         `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsGroup                    *int64                         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot                  *bool                          `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	RuntimeClassName              string                         `json:"runtimeClassName,omitempty" yaml:"runtimeClassName,omitempty"`
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
	ListAll(opts *types.ListOpts) (*DaemonSetCollection, error)
	Create(opts *DaemonSet) (*DaemonSet, error)
	Update(existing *DaemonSet, updates interface{}) (*DaemonSet, error)
	Replace(existing *DaemonSet) (*DaemonSet, error)
	ByID(id string) (*DaemonSet, error)
	Delete(container *DaemonSet) error

	ActionRedeploy(resource *DaemonSet) error
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

func (c *DaemonSetClient) Replace(obj *DaemonSet) (*DaemonSet, error) {
	resp := &DaemonSet{}
	err := c.apiClient.Ops.DoReplace(DaemonSetType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *DaemonSetClient) List(opts *types.ListOpts) (*DaemonSetCollection, error) {
	resp := &DaemonSetCollection{}
	err := c.apiClient.Ops.DoList(DaemonSetType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *DaemonSetClient) ListAll(opts *types.ListOpts) (*DaemonSetCollection, error) {
	resp := &DaemonSetCollection{}
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

func (c *DaemonSetClient) ActionRedeploy(resource *DaemonSet) error {
	err := c.apiClient.Ops.DoAction(DaemonSetType, "redeploy", &resource.Resource, nil, nil)
	return err
}
