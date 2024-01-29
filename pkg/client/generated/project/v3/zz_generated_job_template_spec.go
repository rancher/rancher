package client

const (
	JobTemplateSpecType                               = "jobTemplateSpec"
	JobTemplateSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	JobTemplateSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	JobTemplateSpecFieldBackoffLimitPerIndex          = "backoffLimitPerIndex"
	JobTemplateSpecFieldCompletionMode                = "completionMode"
	JobTemplateSpecFieldContainers                    = "containers"
	JobTemplateSpecFieldDNSConfig                     = "dnsConfig"
	JobTemplateSpecFieldDNSPolicy                     = "dnsPolicy"
	JobTemplateSpecFieldEnableServiceLinks            = "enableServiceLinks"
	JobTemplateSpecFieldEphemeralContainers           = "ephemeralContainers"
	JobTemplateSpecFieldFSGroupChangePolicy           = "fsGroupChangePolicy"
	JobTemplateSpecFieldFsgid                         = "fsgid"
	JobTemplateSpecFieldGids                          = "gids"
	JobTemplateSpecFieldHostAliases                   = "hostAliases"
	JobTemplateSpecFieldHostIPC                       = "hostIPC"
	JobTemplateSpecFieldHostNetwork                   = "hostNetwork"
	JobTemplateSpecFieldHostPID                       = "hostPID"
	JobTemplateSpecFieldHostUsers                     = "hostUsers"
	JobTemplateSpecFieldHostname                      = "hostname"
	JobTemplateSpecFieldImagePullSecrets              = "imagePullSecrets"
	JobTemplateSpecFieldJobConfig                     = "jobConfig"
	JobTemplateSpecFieldJobMetadata                   = "jobMetadata"
	JobTemplateSpecFieldMaxFailedIndexes              = "maxFailedIndexes"
	JobTemplateSpecFieldNodeID                        = "nodeId"
	JobTemplateSpecFieldOS                            = "os"
	JobTemplateSpecFieldObjectMeta                    = "metadata"
	JobTemplateSpecFieldOverhead                      = "overhead"
	JobTemplateSpecFieldPodFailurePolicy              = "podFailurePolicy"
	JobTemplateSpecFieldPodReplacementPolicy          = "podReplacementPolicy"
	JobTemplateSpecFieldPreemptionPolicy              = "preemptionPolicy"
	JobTemplateSpecFieldReadinessGates                = "readinessGates"
	JobTemplateSpecFieldResourceClaims                = "resourceClaims"
	JobTemplateSpecFieldRestartPolicy                 = "restartPolicy"
	JobTemplateSpecFieldRunAsGroup                    = "runAsGroup"
	JobTemplateSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	JobTemplateSpecFieldRuntimeClassName              = "runtimeClassName"
	JobTemplateSpecFieldScheduling                    = "scheduling"
	JobTemplateSpecFieldSchedulingGates               = "schedulingGates"
	JobTemplateSpecFieldSeccompProfile                = "seccompProfile"
	JobTemplateSpecFieldSelector                      = "selector"
	JobTemplateSpecFieldServiceAccountName            = "serviceAccountName"
	JobTemplateSpecFieldSetHostnameAsFQDN             = "setHostnameAsFQDN"
	JobTemplateSpecFieldShareProcessNamespace         = "shareProcessNamespace"
	JobTemplateSpecFieldSubdomain                     = "subdomain"
	JobTemplateSpecFieldSuspend                       = "suspend"
	JobTemplateSpecFieldSysctls                       = "sysctls"
	JobTemplateSpecFieldTTLSecondsAfterFinished       = "ttlSecondsAfterFinished"
	JobTemplateSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	JobTemplateSpecFieldTopologySpreadConstraints     = "topologySpreadConstraints"
	JobTemplateSpecFieldUid                           = "uid"
	JobTemplateSpecFieldVolumes                       = "volumes"
	JobTemplateSpecFieldWindowsOptions                = "windowsOptions"
)

type JobTemplateSpec struct {
	ActiveDeadlineSeconds         *int64                         `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                          `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	BackoffLimitPerIndex          *int64                         `json:"backoffLimitPerIndex,omitempty" yaml:"backoffLimitPerIndex,omitempty"`
	CompletionMode                string                         `json:"completionMode,omitempty" yaml:"completionMode,omitempty"`
	Containers                    []Container                    `json:"containers,omitempty" yaml:"containers,omitempty"`
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
	JobMetadata                   *ObjectMeta                    `json:"jobMetadata,omitempty" yaml:"jobMetadata,omitempty"`
	MaxFailedIndexes              *int64                         `json:"maxFailedIndexes,omitempty" yaml:"maxFailedIndexes,omitempty"`
	NodeID                        string                         `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	OS                            *PodOS                         `json:"os,omitempty" yaml:"os,omitempty"`
	ObjectMeta                    *ObjectMeta                    `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Overhead                      map[string]string              `json:"overhead,omitempty" yaml:"overhead,omitempty"`
	PodFailurePolicy              *PodFailurePolicy              `json:"podFailurePolicy,omitempty" yaml:"podFailurePolicy,omitempty"`
	PodReplacementPolicy          string                         `json:"podReplacementPolicy,omitempty" yaml:"podReplacementPolicy,omitempty"`
	PreemptionPolicy              string                         `json:"preemptionPolicy,omitempty" yaml:"preemptionPolicy,omitempty"`
	ReadinessGates                []PodReadinessGate             `json:"readinessGates,omitempty" yaml:"readinessGates,omitempty"`
	ResourceClaims                []PodResourceClaim             `json:"resourceClaims,omitempty" yaml:"resourceClaims,omitempty"`
	RestartPolicy                 string                         `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsGroup                    *int64                         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot                  *bool                          `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	RuntimeClassName              string                         `json:"runtimeClassName,omitempty" yaml:"runtimeClassName,omitempty"`
	Scheduling                    *Scheduling                    `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	SchedulingGates               []PodSchedulingGate            `json:"schedulingGates,omitempty" yaml:"schedulingGates,omitempty"`
	SeccompProfile                *SeccompProfile                `json:"seccompProfile,omitempty" yaml:"seccompProfile,omitempty"`
	Selector                      *LabelSelector                 `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceAccountName            string                         `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	SetHostnameAsFQDN             *bool                          `json:"setHostnameAsFQDN,omitempty" yaml:"setHostnameAsFQDN,omitempty"`
	ShareProcessNamespace         *bool                          `json:"shareProcessNamespace,omitempty" yaml:"shareProcessNamespace,omitempty"`
	Subdomain                     string                         `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	Suspend                       *bool                          `json:"suspend,omitempty" yaml:"suspend,omitempty"`
	Sysctls                       []Sysctl                       `json:"sysctls,omitempty" yaml:"sysctls,omitempty"`
	TTLSecondsAfterFinished       *int64                         `json:"ttlSecondsAfterFinished,omitempty" yaml:"ttlSecondsAfterFinished,omitempty"`
	TerminationGracePeriodSeconds *int64                         `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	TopologySpreadConstraints     []TopologySpreadConstraint     `json:"topologySpreadConstraints,omitempty" yaml:"topologySpreadConstraints,omitempty"`
	Uid                           *int64                         `json:"uid,omitempty" yaml:"uid,omitempty"`
	Volumes                       []Volume                       `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WindowsOptions                *WindowsSecurityContextOptions `json:"windowsOptions,omitempty" yaml:"windowsOptions,omitempty"`
}
