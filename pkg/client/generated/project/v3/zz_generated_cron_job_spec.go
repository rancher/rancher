package client

const (
	CronJobSpecType                               = "cronJobSpec"
	CronJobSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	CronJobSpecFieldAppArmorProfile               = "appArmorProfile"
	CronJobSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	CronJobSpecFieldBackoffLimitPerIndex          = "backoffLimitPerIndex"
	CronJobSpecFieldCompletionMode                = "completionMode"
	CronJobSpecFieldContainers                    = "containers"
	CronJobSpecFieldCronJobConfig                 = "cronJobConfig"
	CronJobSpecFieldDNSConfig                     = "dnsConfig"
	CronJobSpecFieldDNSPolicy                     = "dnsPolicy"
	CronJobSpecFieldEnableServiceLinks            = "enableServiceLinks"
	CronJobSpecFieldEphemeralContainers           = "ephemeralContainers"
	CronJobSpecFieldFSGroupChangePolicy           = "fsGroupChangePolicy"
	CronJobSpecFieldFsgid                         = "fsgid"
	CronJobSpecFieldGids                          = "gids"
	CronJobSpecFieldHostAliases                   = "hostAliases"
	CronJobSpecFieldHostIPC                       = "hostIPC"
	CronJobSpecFieldHostNetwork                   = "hostNetwork"
	CronJobSpecFieldHostPID                       = "hostPID"
	CronJobSpecFieldHostUsers                     = "hostUsers"
	CronJobSpecFieldHostname                      = "hostname"
	CronJobSpecFieldImagePullSecrets              = "imagePullSecrets"
	CronJobSpecFieldManagedBy                     = "managedBy"
	CronJobSpecFieldMaxFailedIndexes              = "maxFailedIndexes"
	CronJobSpecFieldNodeID                        = "nodeId"
	CronJobSpecFieldOS                            = "os"
	CronJobSpecFieldObjectMeta                    = "metadata"
	CronJobSpecFieldOverhead                      = "overhead"
	CronJobSpecFieldPodFailurePolicy              = "podFailurePolicy"
	CronJobSpecFieldPodReplacementPolicy          = "podReplacementPolicy"
	CronJobSpecFieldPreemptionPolicy              = "preemptionPolicy"
	CronJobSpecFieldReadinessGates                = "readinessGates"
	CronJobSpecFieldResourceClaims                = "resourceClaims"
	CronJobSpecFieldRestartPolicy                 = "restartPolicy"
	CronJobSpecFieldRunAsGroup                    = "runAsGroup"
	CronJobSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	CronJobSpecFieldRuntimeClassName              = "runtimeClassName"
	CronJobSpecFieldScheduling                    = "scheduling"
	CronJobSpecFieldSchedulingGates               = "schedulingGates"
	CronJobSpecFieldSeccompProfile                = "seccompProfile"
	CronJobSpecFieldSelector                      = "selector"
	CronJobSpecFieldServiceAccountName            = "serviceAccountName"
	CronJobSpecFieldSetHostnameAsFQDN             = "setHostnameAsFQDN"
	CronJobSpecFieldShareProcessNamespace         = "shareProcessNamespace"
	CronJobSpecFieldSubdomain                     = "subdomain"
	CronJobSpecFieldSuccessPolicy                 = "successPolicy"
	CronJobSpecFieldSysctls                       = "sysctls"
	CronJobSpecFieldTTLSecondsAfterFinished       = "ttlSecondsAfterFinished"
	CronJobSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	CronJobSpecFieldTimeZone                      = "timeZone"
	CronJobSpecFieldTopologySpreadConstraints     = "topologySpreadConstraints"
	CronJobSpecFieldUid                           = "uid"
	CronJobSpecFieldVolumes                       = "volumes"
	CronJobSpecFieldWindowsOptions                = "windowsOptions"
)

type CronJobSpec struct {
	ActiveDeadlineSeconds         *int64                         `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	AppArmorProfile               *AppArmorProfile               `json:"appArmorProfile,omitempty" yaml:"appArmorProfile,omitempty"`
	AutomountServiceAccountToken  *bool                          `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	BackoffLimitPerIndex          *int64                         `json:"backoffLimitPerIndex,omitempty" yaml:"backoffLimitPerIndex,omitempty"`
	CompletionMode                string                         `json:"completionMode,omitempty" yaml:"completionMode,omitempty"`
	Containers                    []Container                    `json:"containers,omitempty" yaml:"containers,omitempty"`
	CronJobConfig                 *CronJobConfig                 `json:"cronJobConfig,omitempty" yaml:"cronJobConfig,omitempty"`
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
	ManagedBy                     string                         `json:"managedBy,omitempty" yaml:"managedBy,omitempty"`
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
	SuccessPolicy                 *SuccessPolicy                 `json:"successPolicy,omitempty" yaml:"successPolicy,omitempty"`
	Sysctls                       []Sysctl                       `json:"sysctls,omitempty" yaml:"sysctls,omitempty"`
	TTLSecondsAfterFinished       *int64                         `json:"ttlSecondsAfterFinished,omitempty" yaml:"ttlSecondsAfterFinished,omitempty"`
	TerminationGracePeriodSeconds *int64                         `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	TimeZone                      string                         `json:"timeZone,omitempty" yaml:"timeZone,omitempty"`
	TopologySpreadConstraints     []TopologySpreadConstraint     `json:"topologySpreadConstraints,omitempty" yaml:"topologySpreadConstraints,omitempty"`
	Uid                           *int64                         `json:"uid,omitempty" yaml:"uid,omitempty"`
	Volumes                       []Volume                       `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WindowsOptions                *WindowsSecurityContextOptions `json:"windowsOptions,omitempty" yaml:"windowsOptions,omitempty"`
}
