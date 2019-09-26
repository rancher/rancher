package client

import (
	"github.com/rancher/norman/types"
)

const (
	CronJobType                               = "cronJob"
	CronJobFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	CronJobFieldAnnotations                   = "annotations"
	CronJobFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	CronJobFieldContainers                    = "containers"
	CronJobFieldCreated                       = "created"
	CronJobFieldCreatorID                     = "creatorId"
	CronJobFieldCronJobConfig                 = "cronJobConfig"
	CronJobFieldCronJobStatus                 = "cronJobStatus"
	CronJobFieldDNSConfig                     = "dnsConfig"
	CronJobFieldDNSPolicy                     = "dnsPolicy"
	CronJobFieldEnableServiceLinks            = "enableServiceLinks"
	CronJobFieldEphemeralContainers           = "ephemeralContainers"
	CronJobFieldFsgid                         = "fsgid"
	CronJobFieldGids                          = "gids"
	CronJobFieldHostAliases                   = "hostAliases"
	CronJobFieldHostIPC                       = "hostIPC"
	CronJobFieldHostNetwork                   = "hostNetwork"
	CronJobFieldHostPID                       = "hostPID"
	CronJobFieldHostname                      = "hostname"
	CronJobFieldImagePullSecrets              = "imagePullSecrets"
	CronJobFieldLabels                        = "labels"
	CronJobFieldName                          = "name"
	CronJobFieldNamespaceId                   = "namespaceId"
	CronJobFieldNodeID                        = "nodeId"
	CronJobFieldOverhead                      = "overhead"
	CronJobFieldOwnerReferences               = "ownerReferences"
	CronJobFieldPreemptionPolicy              = "preemptionPolicy"
	CronJobFieldProjectID                     = "projectId"
	CronJobFieldPublicEndpoints               = "publicEndpoints"
	CronJobFieldReadinessGates                = "readinessGates"
	CronJobFieldRemoved                       = "removed"
	CronJobFieldRestartPolicy                 = "restartPolicy"
	CronJobFieldRunAsGroup                    = "runAsGroup"
	CronJobFieldRunAsNonRoot                  = "runAsNonRoot"
	CronJobFieldRuntimeClassName              = "runtimeClassName"
	CronJobFieldScheduling                    = "scheduling"
	CronJobFieldSelector                      = "selector"
	CronJobFieldServiceAccountName            = "serviceAccountName"
	CronJobFieldShareProcessNamespace         = "shareProcessNamespace"
	CronJobFieldState                         = "state"
	CronJobFieldSubdomain                     = "subdomain"
	CronJobFieldSysctls                       = "sysctls"
	CronJobFieldTTLSecondsAfterFinished       = "ttlSecondsAfterFinished"
	CronJobFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	CronJobFieldTopologySpreadConstraints     = "topologySpreadConstraints"
	CronJobFieldTransitioning                 = "transitioning"
	CronJobFieldTransitioningMessage          = "transitioningMessage"
	CronJobFieldUUID                          = "uuid"
	CronJobFieldUid                           = "uid"
	CronJobFieldVolumes                       = "volumes"
	CronJobFieldWindowsOptions                = "windowsOptions"
	CronJobFieldWorkloadAnnotations           = "workloadAnnotations"
	CronJobFieldWorkloadLabels                = "workloadLabels"
	CronJobFieldWorkloadMetrics               = "workloadMetrics"
)

type CronJob struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                         `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string              `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AutomountServiceAccountToken  *bool                          `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	Containers                    []Container                    `json:"containers,omitempty" yaml:"containers,omitempty"`
	Created                       string                         `json:"created,omitempty" yaml:"created,omitempty"`
	CreatorID                     string                         `json:"creatorId,omitempty" yaml:"creatorId,omitempty"`
	CronJobConfig                 *CronJobConfig                 `json:"cronJobConfig,omitempty" yaml:"cronJobConfig,omitempty"`
	CronJobStatus                 *CronJobStatus                 `json:"cronJobStatus,omitempty" yaml:"cronJobStatus,omitempty"`
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
	TTLSecondsAfterFinished       *int64                         `json:"ttlSecondsAfterFinished,omitempty" yaml:"ttlSecondsAfterFinished,omitempty"`
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

type CronJobCollection struct {
	types.Collection
	Data   []CronJob `json:"data,omitempty"`
	client *CronJobClient
}

type CronJobClient struct {
	apiClient *Client
}

type CronJobOperations interface {
	List(opts *types.ListOpts) (*CronJobCollection, error)
	Create(opts *CronJob) (*CronJob, error)
	Update(existing *CronJob, updates interface{}) (*CronJob, error)
	Replace(existing *CronJob) (*CronJob, error)
	ByID(id string) (*CronJob, error)
	Delete(container *CronJob) error
}

func newCronJobClient(apiClient *Client) *CronJobClient {
	return &CronJobClient{
		apiClient: apiClient,
	}
}

func (c *CronJobClient) Create(container *CronJob) (*CronJob, error) {
	resp := &CronJob{}
	err := c.apiClient.Ops.DoCreate(CronJobType, container, resp)
	return resp, err
}

func (c *CronJobClient) Update(existing *CronJob, updates interface{}) (*CronJob, error) {
	resp := &CronJob{}
	err := c.apiClient.Ops.DoUpdate(CronJobType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *CronJobClient) Replace(obj *CronJob) (*CronJob, error) {
	resp := &CronJob{}
	err := c.apiClient.Ops.DoReplace(CronJobType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *CronJobClient) List(opts *types.ListOpts) (*CronJobCollection, error) {
	resp := &CronJobCollection{}
	err := c.apiClient.Ops.DoList(CronJobType, opts, resp)
	resp.client = c
	return resp, err
}

func (cc *CronJobCollection) Next() (*CronJobCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &CronJobCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *CronJobClient) ByID(id string) (*CronJob, error) {
	resp := &CronJob{}
	err := c.apiClient.Ops.DoByID(CronJobType, id, resp)
	return resp, err
}

func (c *CronJobClient) Delete(container *CronJob) error {
	return c.apiClient.Ops.DoResourceDelete(CronJobType, &container.Resource)
}
