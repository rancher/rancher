package client

import (
	"github.com/rancher/norman/types"
)

const (
	JobType                               = "job"
	JobFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	JobFieldAnnotations                   = "annotations"
	JobFieldAppArmorProfile               = "appArmorProfile"
	JobFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	JobFieldBackoffLimitPerIndex          = "backoffLimitPerIndex"
	JobFieldCompletionMode                = "completionMode"
	JobFieldContainers                    = "containers"
	JobFieldCreated                       = "created"
	JobFieldCreatorID                     = "creatorId"
	JobFieldDNSConfig                     = "dnsConfig"
	JobFieldDNSPolicy                     = "dnsPolicy"
	JobFieldEnableServiceLinks            = "enableServiceLinks"
	JobFieldEphemeralContainers           = "ephemeralContainers"
	JobFieldFSGroupChangePolicy           = "fsGroupChangePolicy"
	JobFieldFsgid                         = "fsgid"
	JobFieldGids                          = "gids"
	JobFieldHostAliases                   = "hostAliases"
	JobFieldHostIPC                       = "hostIPC"
	JobFieldHostNetwork                   = "hostNetwork"
	JobFieldHostPID                       = "hostPID"
	JobFieldHostUsers                     = "hostUsers"
	JobFieldHostname                      = "hostname"
	JobFieldImagePullSecrets              = "imagePullSecrets"
	JobFieldJobConfig                     = "jobConfig"
	JobFieldJobStatus                     = "jobStatus"
	JobFieldLabels                        = "labels"
	JobFieldManagedBy                     = "managedBy"
	JobFieldMaxFailedIndexes              = "maxFailedIndexes"
	JobFieldName                          = "name"
	JobFieldNamespaceId                   = "namespaceId"
	JobFieldNodeID                        = "nodeId"
	JobFieldOS                            = "os"
	JobFieldOverhead                      = "overhead"
	JobFieldOwnerReferences               = "ownerReferences"
	JobFieldPodFailurePolicy              = "podFailurePolicy"
	JobFieldPodReplacementPolicy          = "podReplacementPolicy"
	JobFieldPreemptionPolicy              = "preemptionPolicy"
	JobFieldProjectID                     = "projectId"
	JobFieldPublicEndpoints               = "publicEndpoints"
	JobFieldReadinessGates                = "readinessGates"
	JobFieldRemoved                       = "removed"
	JobFieldResourceClaims                = "resourceClaims"
	JobFieldResources                     = "resources"
	JobFieldRestartPolicy                 = "restartPolicy"
	JobFieldRunAsGroup                    = "runAsGroup"
	JobFieldRunAsNonRoot                  = "runAsNonRoot"
	JobFieldRuntimeClassName              = "runtimeClassName"
	JobFieldSELinuxChangePolicy           = "seLinuxChangePolicy"
	JobFieldScheduling                    = "scheduling"
	JobFieldSchedulingGates               = "schedulingGates"
	JobFieldSeccompProfile                = "seccompProfile"
	JobFieldSelector                      = "selector"
	JobFieldServiceAccountName            = "serviceAccountName"
	JobFieldSetHostnameAsFQDN             = "setHostnameAsFQDN"
	JobFieldShareProcessNamespace         = "shareProcessNamespace"
	JobFieldState                         = "state"
	JobFieldSubdomain                     = "subdomain"
	JobFieldSuccessPolicy                 = "successPolicy"
	JobFieldSupplementalGroupsPolicy      = "supplementalGroupsPolicy"
	JobFieldSuspend                       = "suspend"
	JobFieldSysctls                       = "sysctls"
	JobFieldTTLSecondsAfterFinished       = "ttlSecondsAfterFinished"
	JobFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	JobFieldTopologySpreadConstraints     = "topologySpreadConstraints"
	JobFieldTransitioning                 = "transitioning"
	JobFieldTransitioningMessage          = "transitioningMessage"
	JobFieldUUID                          = "uuid"
	JobFieldUid                           = "uid"
	JobFieldVolumes                       = "volumes"
	JobFieldWindowsOptions                = "windowsOptions"
	JobFieldWorkloadAnnotations           = "workloadAnnotations"
	JobFieldWorkloadLabels                = "workloadLabels"
	JobFieldWorkloadMetrics               = "workloadMetrics"
)

type Job struct {
	types.Resource
	ActiveDeadlineSeconds         *int64                         `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	Annotations                   map[string]string              `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	AppArmorProfile               *AppArmorProfile               `json:"appArmorProfile,omitempty" yaml:"appArmorProfile,omitempty"`
	AutomountServiceAccountToken  *bool                          `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	BackoffLimitPerIndex          *int64                         `json:"backoffLimitPerIndex,omitempty" yaml:"backoffLimitPerIndex,omitempty"`
	CompletionMode                string                         `json:"completionMode,omitempty" yaml:"completionMode,omitempty"`
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
	HostUsers                     *bool                          `json:"hostUsers,omitempty" yaml:"hostUsers,omitempty"`
	Hostname                      string                         `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference         `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	JobConfig                     *JobConfig                     `json:"jobConfig,omitempty" yaml:"jobConfig,omitempty"`
	JobStatus                     *JobStatus                     `json:"jobStatus,omitempty" yaml:"jobStatus,omitempty"`
	Labels                        map[string]string              `json:"labels,omitempty" yaml:"labels,omitempty"`
	ManagedBy                     string                         `json:"managedBy,omitempty" yaml:"managedBy,omitempty"`
	MaxFailedIndexes              *int64                         `json:"maxFailedIndexes,omitempty" yaml:"maxFailedIndexes,omitempty"`
	Name                          string                         `json:"name,omitempty" yaml:"name,omitempty"`
	NamespaceId                   string                         `json:"namespaceId,omitempty" yaml:"namespaceId,omitempty"`
	NodeID                        string                         `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	OS                            *PodOS                         `json:"os,omitempty" yaml:"os,omitempty"`
	Overhead                      map[string]string              `json:"overhead,omitempty" yaml:"overhead,omitempty"`
	OwnerReferences               []OwnerReference               `json:"ownerReferences,omitempty" yaml:"ownerReferences,omitempty"`
	PodFailurePolicy              *PodFailurePolicy              `json:"podFailurePolicy,omitempty" yaml:"podFailurePolicy,omitempty"`
	PodReplacementPolicy          string                         `json:"podReplacementPolicy,omitempty" yaml:"podReplacementPolicy,omitempty"`
	PreemptionPolicy              string                         `json:"preemptionPolicy,omitempty" yaml:"preemptionPolicy,omitempty"`
	ProjectID                     string                         `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	PublicEndpoints               []PublicEndpoint               `json:"publicEndpoints,omitempty" yaml:"publicEndpoints,omitempty"`
	ReadinessGates                []PodReadinessGate             `json:"readinessGates,omitempty" yaml:"readinessGates,omitempty"`
	Removed                       string                         `json:"removed,omitempty" yaml:"removed,omitempty"`
	ResourceClaims                []PodResourceClaim             `json:"resourceClaims,omitempty" yaml:"resourceClaims,omitempty"`
	Resources                     *ResourceRequirements          `json:"resources,omitempty" yaml:"resources,omitempty"`
	RestartPolicy                 string                         `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsGroup                    *int64                         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot                  *bool                          `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	RuntimeClassName              string                         `json:"runtimeClassName,omitempty" yaml:"runtimeClassName,omitempty"`
	SELinuxChangePolicy           string                         `json:"seLinuxChangePolicy,omitempty" yaml:"seLinuxChangePolicy,omitempty"`
	Scheduling                    *Scheduling                    `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	SchedulingGates               []PodSchedulingGate            `json:"schedulingGates,omitempty" yaml:"schedulingGates,omitempty"`
	SeccompProfile                *SeccompProfile                `json:"seccompProfile,omitempty" yaml:"seccompProfile,omitempty"`
	Selector                      *LabelSelector                 `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceAccountName            string                         `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	SetHostnameAsFQDN             *bool                          `json:"setHostnameAsFQDN,omitempty" yaml:"setHostnameAsFQDN,omitempty"`
	ShareProcessNamespace         *bool                          `json:"shareProcessNamespace,omitempty" yaml:"shareProcessNamespace,omitempty"`
	State                         string                         `json:"state,omitempty" yaml:"state,omitempty"`
	Subdomain                     string                         `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	SuccessPolicy                 *SuccessPolicy                 `json:"successPolicy,omitempty" yaml:"successPolicy,omitempty"`
	SupplementalGroupsPolicy      string                         `json:"supplementalGroupsPolicy,omitempty" yaml:"supplementalGroupsPolicy,omitempty"`
	Suspend                       *bool                          `json:"suspend,omitempty" yaml:"suspend,omitempty"`
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

type JobCollection struct {
	types.Collection
	Data   []Job `json:"data,omitempty"`
	client *JobClient
}

type JobClient struct {
	apiClient *Client
}

type JobOperations interface {
	List(opts *types.ListOpts) (*JobCollection, error)
	ListAll(opts *types.ListOpts) (*JobCollection, error)
	Create(opts *Job) (*Job, error)
	Update(existing *Job, updates interface{}) (*Job, error)
	Replace(existing *Job) (*Job, error)
	ByID(id string) (*Job, error)
	Delete(container *Job) error
}

func newJobClient(apiClient *Client) *JobClient {
	return &JobClient{
		apiClient: apiClient,
	}
}

func (c *JobClient) Create(container *Job) (*Job, error) {
	resp := &Job{}
	err := c.apiClient.Ops.DoCreate(JobType, container, resp)
	return resp, err
}

func (c *JobClient) Update(existing *Job, updates interface{}) (*Job, error) {
	resp := &Job{}
	err := c.apiClient.Ops.DoUpdate(JobType, &existing.Resource, updates, resp)
	return resp, err
}

func (c *JobClient) Replace(obj *Job) (*Job, error) {
	resp := &Job{}
	err := c.apiClient.Ops.DoReplace(JobType, &obj.Resource, obj, resp)
	return resp, err
}

func (c *JobClient) List(opts *types.ListOpts) (*JobCollection, error) {
	resp := &JobCollection{}
	err := c.apiClient.Ops.DoList(JobType, opts, resp)
	resp.client = c
	return resp, err
}

func (c *JobClient) ListAll(opts *types.ListOpts) (*JobCollection, error) {
	resp := &JobCollection{}
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

func (cc *JobCollection) Next() (*JobCollection, error) {
	if cc != nil && cc.Pagination != nil && cc.Pagination.Next != "" {
		resp := &JobCollection{}
		err := cc.client.apiClient.Ops.DoNext(cc.Pagination.Next, resp)
		resp.client = cc.client
		return resp, err
	}
	return nil, nil
}

func (c *JobClient) ByID(id string) (*Job, error) {
	resp := &Job{}
	err := c.apiClient.Ops.DoByID(JobType, id, resp)
	return resp, err
}

func (c *JobClient) Delete(container *Job) error {
	return c.apiClient.Ops.DoResourceDelete(JobType, &container.Resource)
}
