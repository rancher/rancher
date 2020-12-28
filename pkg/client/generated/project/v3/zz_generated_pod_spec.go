package client

const (
	PodSpecType                               = "podSpec"
	PodSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	PodSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	PodSpecFieldContainers                    = "containers"
	PodSpecFieldDNSConfig                     = "dnsConfig"
	PodSpecFieldDNSPolicy                     = "dnsPolicy"
	PodSpecFieldEnableServiceLinks            = "enableServiceLinks"
	PodSpecFieldEphemeralContainers           = "ephemeralContainers"
	PodSpecFieldFSGroupChangePolicy           = "fsGroupChangePolicy"
	PodSpecFieldFsgid                         = "fsgid"
	PodSpecFieldGids                          = "gids"
	PodSpecFieldHostAliases                   = "hostAliases"
	PodSpecFieldHostIPC                       = "hostIPC"
	PodSpecFieldHostNetwork                   = "hostNetwork"
	PodSpecFieldHostPID                       = "hostPID"
	PodSpecFieldHostname                      = "hostname"
	PodSpecFieldImagePullSecrets              = "imagePullSecrets"
	PodSpecFieldNodeID                        = "nodeId"
	PodSpecFieldOverhead                      = "overhead"
	PodSpecFieldPreemptionPolicy              = "preemptionPolicy"
	PodSpecFieldReadinessGates                = "readinessGates"
	PodSpecFieldRestartPolicy                 = "restartPolicy"
	PodSpecFieldRunAsGroup                    = "runAsGroup"
	PodSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	PodSpecFieldRuntimeClassName              = "runtimeClassName"
	PodSpecFieldScheduling                    = "scheduling"
	PodSpecFieldSeccompProfile                = "seccompProfile"
	PodSpecFieldServiceAccountName            = "serviceAccountName"
	PodSpecFieldSetHostnameAsFQDN             = "setHostnameAsFQDN"
	PodSpecFieldShareProcessNamespace         = "shareProcessNamespace"
	PodSpecFieldSubdomain                     = "subdomain"
	PodSpecFieldSysctls                       = "sysctls"
	PodSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	PodSpecFieldTopologySpreadConstraints     = "topologySpreadConstraints"
	PodSpecFieldUid                           = "uid"
	PodSpecFieldVolumes                       = "volumes"
	PodSpecFieldWindowsOptions                = "windowsOptions"
)

type PodSpec struct {
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
	NodeID                        string                         `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	Overhead                      map[string]string              `json:"overhead,omitempty" yaml:"overhead,omitempty"`
	PreemptionPolicy              string                         `json:"preemptionPolicy,omitempty" yaml:"preemptionPolicy,omitempty"`
	ReadinessGates                []PodReadinessGate             `json:"readinessGates,omitempty" yaml:"readinessGates,omitempty"`
	RestartPolicy                 string                         `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsGroup                    *int64                         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot                  *bool                          `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	RuntimeClassName              string                         `json:"runtimeClassName,omitempty" yaml:"runtimeClassName,omitempty"`
	Scheduling                    *Scheduling                    `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	SeccompProfile                *SeccompProfile                `json:"seccompProfile,omitempty" yaml:"seccompProfile,omitempty"`
	ServiceAccountName            string                         `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	SetHostnameAsFQDN             *bool                          `json:"setHostnameAsFQDN,omitempty" yaml:"setHostnameAsFQDN,omitempty"`
	ShareProcessNamespace         *bool                          `json:"shareProcessNamespace,omitempty" yaml:"shareProcessNamespace,omitempty"`
	Subdomain                     string                         `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	Sysctls                       []Sysctl                       `json:"sysctls,omitempty" yaml:"sysctls,omitempty"`
	TerminationGracePeriodSeconds *int64                         `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	TopologySpreadConstraints     []TopologySpreadConstraint     `json:"topologySpreadConstraints,omitempty" yaml:"topologySpreadConstraints,omitempty"`
	Uid                           *int64                         `json:"uid,omitempty" yaml:"uid,omitempty"`
	Volumes                       []Volume                       `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WindowsOptions                *WindowsSecurityContextOptions `json:"windowsOptions,omitempty" yaml:"windowsOptions,omitempty"`
}
