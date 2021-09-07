package client

const (
	ReplicationControllerSpecType                               = "replicationControllerSpec"
	ReplicationControllerSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	ReplicationControllerSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	ReplicationControllerSpecFieldContainers                    = "containers"
	ReplicationControllerSpecFieldDNSConfig                     = "dnsConfig"
	ReplicationControllerSpecFieldDNSPolicy                     = "dnsPolicy"
	ReplicationControllerSpecFieldEnableServiceLinks            = "enableServiceLinks"
	ReplicationControllerSpecFieldEphemeralContainers           = "ephemeralContainers"
	ReplicationControllerSpecFieldFSGroupChangePolicy           = "fsGroupChangePolicy"
	ReplicationControllerSpecFieldFsgid                         = "fsgid"
	ReplicationControllerSpecFieldGids                          = "gids"
	ReplicationControllerSpecFieldHostAliases                   = "hostAliases"
	ReplicationControllerSpecFieldHostIPC                       = "hostIPC"
	ReplicationControllerSpecFieldHostNetwork                   = "hostNetwork"
	ReplicationControllerSpecFieldHostPID                       = "hostPID"
	ReplicationControllerSpecFieldHostname                      = "hostname"
	ReplicationControllerSpecFieldImagePullSecrets              = "imagePullSecrets"
	ReplicationControllerSpecFieldNodeID                        = "nodeId"
	ReplicationControllerSpecFieldObjectMeta                    = "metadata"
	ReplicationControllerSpecFieldOverhead                      = "overhead"
	ReplicationControllerSpecFieldPreemptionPolicy              = "preemptionPolicy"
	ReplicationControllerSpecFieldReadinessGates                = "readinessGates"
	ReplicationControllerSpecFieldReplicationControllerConfig   = "replicationControllerConfig"
	ReplicationControllerSpecFieldRestartPolicy                 = "restartPolicy"
	ReplicationControllerSpecFieldRunAsGroup                    = "runAsGroup"
	ReplicationControllerSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	ReplicationControllerSpecFieldRuntimeClassName              = "runtimeClassName"
	ReplicationControllerSpecFieldScale                         = "scale"
	ReplicationControllerSpecFieldScheduling                    = "scheduling"
	ReplicationControllerSpecFieldSeccompProfile                = "seccompProfile"
	ReplicationControllerSpecFieldSelector                      = "selector"
	ReplicationControllerSpecFieldServiceAccountName            = "serviceAccountName"
	ReplicationControllerSpecFieldSetHostnameAsFQDN             = "setHostnameAsFQDN"
	ReplicationControllerSpecFieldShareProcessNamespace         = "shareProcessNamespace"
	ReplicationControllerSpecFieldSubdomain                     = "subdomain"
	ReplicationControllerSpecFieldSysctls                       = "sysctls"
	ReplicationControllerSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	ReplicationControllerSpecFieldTopologySpreadConstraints     = "topologySpreadConstraints"
	ReplicationControllerSpecFieldUid                           = "uid"
	ReplicationControllerSpecFieldVolumes                       = "volumes"
	ReplicationControllerSpecFieldWindowsOptions                = "windowsOptions"
)

type ReplicationControllerSpec struct {
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
	ObjectMeta                    *ObjectMeta                    `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Overhead                      map[string]string              `json:"overhead,omitempty" yaml:"overhead,omitempty"`
	PreemptionPolicy              string                         `json:"preemptionPolicy,omitempty" yaml:"preemptionPolicy,omitempty"`
	ReadinessGates                []PodReadinessGate             `json:"readinessGates,omitempty" yaml:"readinessGates,omitempty"`
	ReplicationControllerConfig   *ReplicationControllerConfig   `json:"replicationControllerConfig,omitempty" yaml:"replicationControllerConfig,omitempty"`
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
	Subdomain                     string                         `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	Sysctls                       []Sysctl                       `json:"sysctls,omitempty" yaml:"sysctls,omitempty"`
	TerminationGracePeriodSeconds *int64                         `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	TopologySpreadConstraints     []TopologySpreadConstraint     `json:"topologySpreadConstraints,omitempty" yaml:"topologySpreadConstraints,omitempty"`
	Uid                           *int64                         `json:"uid,omitempty" yaml:"uid,omitempty"`
	Volumes                       []Volume                       `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WindowsOptions                *WindowsSecurityContextOptions `json:"windowsOptions,omitempty" yaml:"windowsOptions,omitempty"`
}
