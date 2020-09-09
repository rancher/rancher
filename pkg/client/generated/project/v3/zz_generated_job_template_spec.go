package client

const (
	JobTemplateSpecType                               = "jobTemplateSpec"
	JobTemplateSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	JobTemplateSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
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
	JobTemplateSpecFieldHostname                      = "hostname"
	JobTemplateSpecFieldImagePullSecrets              = "imagePullSecrets"
	JobTemplateSpecFieldJobConfig                     = "jobConfig"
	JobTemplateSpecFieldJobMetadata                   = "jobMetadata"
	JobTemplateSpecFieldNodeID                        = "nodeId"
	JobTemplateSpecFieldObjectMeta                    = "metadata"
	JobTemplateSpecFieldOverhead                      = "overhead"
	JobTemplateSpecFieldPreemptionPolicy              = "preemptionPolicy"
	JobTemplateSpecFieldReadinessGates                = "readinessGates"
	JobTemplateSpecFieldRestartPolicy                 = "restartPolicy"
	JobTemplateSpecFieldRunAsGroup                    = "runAsGroup"
	JobTemplateSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	JobTemplateSpecFieldRuntimeClassName              = "runtimeClassName"
	JobTemplateSpecFieldScheduling                    = "scheduling"
	JobTemplateSpecFieldSeccompProfile                = "seccompProfile"
	JobTemplateSpecFieldSelector                      = "selector"
	JobTemplateSpecFieldServiceAccountName            = "serviceAccountName"
	JobTemplateSpecFieldSetHostnameAsFQDN             = "setHostnameAsFQDN"
	JobTemplateSpecFieldShareProcessNamespace         = "shareProcessNamespace"
	JobTemplateSpecFieldSubdomain                     = "subdomain"
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
	Hostname                      string                         `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference         `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	JobConfig                     *JobConfig                     `json:"jobConfig,omitempty" yaml:"jobConfig,omitempty"`
	JobMetadata                   *ObjectMeta                    `json:"jobMetadata,omitempty" yaml:"jobMetadata,omitempty"`
	NodeID                        string                         `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta                    `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Overhead                      map[string]string              `json:"overhead,omitempty" yaml:"overhead,omitempty"`
	PreemptionPolicy              string                         `json:"preemptionPolicy,omitempty" yaml:"preemptionPolicy,omitempty"`
	ReadinessGates                []PodReadinessGate             `json:"readinessGates,omitempty" yaml:"readinessGates,omitempty"`
	RestartPolicy                 string                         `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsGroup                    *int64                         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot                  *bool                          `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	RuntimeClassName              string                         `json:"runtimeClassName,omitempty" yaml:"runtimeClassName,omitempty"`
	Scheduling                    *Scheduling                    `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	SeccompProfile                *SeccompProfile                `json:"seccompProfile,omitempty" yaml:"seccompProfile,omitempty"`
	Selector                      *LabelSelector                 `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceAccountName            string                         `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	SetHostnameAsFQDN             *bool                          `json:"setHostnameAsFQDN,omitempty" yaml:"setHostnameAsFQDN,omitempty"`
	ShareProcessNamespace         *bool                          `json:"shareProcessNamespace,omitempty" yaml:"shareProcessNamespace,omitempty"`
	Subdomain                     string                         `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	Sysctls                       []Sysctl                       `json:"sysctls,omitempty" yaml:"sysctls,omitempty"`
	TTLSecondsAfterFinished       *int64                         `json:"ttlSecondsAfterFinished,omitempty" yaml:"ttlSecondsAfterFinished,omitempty"`
	TerminationGracePeriodSeconds *int64                         `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	TopologySpreadConstraints     []TopologySpreadConstraint     `json:"topologySpreadConstraints,omitempty" yaml:"topologySpreadConstraints,omitempty"`
	Uid                           *int64                         `json:"uid,omitempty" yaml:"uid,omitempty"`
	Volumes                       []Volume                       `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WindowsOptions                *WindowsSecurityContextOptions `json:"windowsOptions,omitempty" yaml:"windowsOptions,omitempty"`
}
