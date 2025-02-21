package client

const (
	DeploymentSpecType                               = "deploymentSpec"
	DeploymentSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	DeploymentSpecFieldAppArmorProfile               = "appArmorProfile"
	DeploymentSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	DeploymentSpecFieldContainers                    = "containers"
	DeploymentSpecFieldDNSConfig                     = "dnsConfig"
	DeploymentSpecFieldDNSPolicy                     = "dnsPolicy"
	DeploymentSpecFieldDeploymentConfig              = "deploymentConfig"
	DeploymentSpecFieldEnableServiceLinks            = "enableServiceLinks"
	DeploymentSpecFieldEphemeralContainers           = "ephemeralContainers"
	DeploymentSpecFieldFSGroupChangePolicy           = "fsGroupChangePolicy"
	DeploymentSpecFieldFsgid                         = "fsgid"
	DeploymentSpecFieldGids                          = "gids"
	DeploymentSpecFieldHostAliases                   = "hostAliases"
	DeploymentSpecFieldHostIPC                       = "hostIPC"
	DeploymentSpecFieldHostNetwork                   = "hostNetwork"
	DeploymentSpecFieldHostPID                       = "hostPID"
	DeploymentSpecFieldHostUsers                     = "hostUsers"
	DeploymentSpecFieldHostname                      = "hostname"
	DeploymentSpecFieldImagePullSecrets              = "imagePullSecrets"
	DeploymentSpecFieldNodeID                        = "nodeId"
	DeploymentSpecFieldOS                            = "os"
	DeploymentSpecFieldObjectMeta                    = "metadata"
	DeploymentSpecFieldOverhead                      = "overhead"
	DeploymentSpecFieldPaused                        = "paused"
	DeploymentSpecFieldPreemptionPolicy              = "preemptionPolicy"
	DeploymentSpecFieldReadinessGates                = "readinessGates"
	DeploymentSpecFieldResourceClaims                = "resourceClaims"
	DeploymentSpecFieldResources                     = "resources"
	DeploymentSpecFieldRestartPolicy                 = "restartPolicy"
	DeploymentSpecFieldRunAsGroup                    = "runAsGroup"
	DeploymentSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	DeploymentSpecFieldRuntimeClassName              = "runtimeClassName"
	DeploymentSpecFieldSELinuxChangePolicy           = "seLinuxChangePolicy"
	DeploymentSpecFieldScale                         = "scale"
	DeploymentSpecFieldScheduling                    = "scheduling"
	DeploymentSpecFieldSchedulingGates               = "schedulingGates"
	DeploymentSpecFieldSeccompProfile                = "seccompProfile"
	DeploymentSpecFieldSelector                      = "selector"
	DeploymentSpecFieldServiceAccountName            = "serviceAccountName"
	DeploymentSpecFieldSetHostnameAsFQDN             = "setHostnameAsFQDN"
	DeploymentSpecFieldShareProcessNamespace         = "shareProcessNamespace"
	DeploymentSpecFieldSubdomain                     = "subdomain"
	DeploymentSpecFieldSupplementalGroupsPolicy      = "supplementalGroupsPolicy"
	DeploymentSpecFieldSysctls                       = "sysctls"
	DeploymentSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	DeploymentSpecFieldTopologySpreadConstraints     = "topologySpreadConstraints"
	DeploymentSpecFieldUid                           = "uid"
	DeploymentSpecFieldVolumes                       = "volumes"
	DeploymentSpecFieldWindowsOptions                = "windowsOptions"
)

type DeploymentSpec struct {
	ActiveDeadlineSeconds         *int64                         `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	AppArmorProfile               *AppArmorProfile               `json:"appArmorProfile,omitempty" yaml:"appArmorProfile,omitempty"`
	AutomountServiceAccountToken  *bool                          `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	Containers                    []Container                    `json:"containers,omitempty" yaml:"containers,omitempty"`
	DNSConfig                     *PodDNSConfig                  `json:"dnsConfig,omitempty" yaml:"dnsConfig,omitempty"`
	DNSPolicy                     string                         `json:"dnsPolicy,omitempty" yaml:"dnsPolicy,omitempty"`
	DeploymentConfig              *DeploymentConfig              `json:"deploymentConfig,omitempty" yaml:"deploymentConfig,omitempty"`
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
	NodeID                        string                         `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	OS                            *PodOS                         `json:"os,omitempty" yaml:"os,omitempty"`
	ObjectMeta                    *ObjectMeta                    `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Overhead                      map[string]string              `json:"overhead,omitempty" yaml:"overhead,omitempty"`
	Paused                        bool                           `json:"paused,omitempty" yaml:"paused,omitempty"`
	PreemptionPolicy              string                         `json:"preemptionPolicy,omitempty" yaml:"preemptionPolicy,omitempty"`
	ReadinessGates                []PodReadinessGate             `json:"readinessGates,omitempty" yaml:"readinessGates,omitempty"`
	ResourceClaims                []PodResourceClaim             `json:"resourceClaims,omitempty" yaml:"resourceClaims,omitempty"`
	Resources                     *ResourceRequirements          `json:"resources,omitempty" yaml:"resources,omitempty"`
	RestartPolicy                 string                         `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsGroup                    *int64                         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot                  *bool                          `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	RuntimeClassName              string                         `json:"runtimeClassName,omitempty" yaml:"runtimeClassName,omitempty"`
	SELinuxChangePolicy           string                         `json:"seLinuxChangePolicy,omitempty" yaml:"seLinuxChangePolicy,omitempty"`
	Scale                         *int64                         `json:"scale,omitempty" yaml:"scale,omitempty"`
	Scheduling                    *Scheduling                    `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	SchedulingGates               []PodSchedulingGate            `json:"schedulingGates,omitempty" yaml:"schedulingGates,omitempty"`
	SeccompProfile                *SeccompProfile                `json:"seccompProfile,omitempty" yaml:"seccompProfile,omitempty"`
	Selector                      *LabelSelector                 `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceAccountName            string                         `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	SetHostnameAsFQDN             *bool                          `json:"setHostnameAsFQDN,omitempty" yaml:"setHostnameAsFQDN,omitempty"`
	ShareProcessNamespace         *bool                          `json:"shareProcessNamespace,omitempty" yaml:"shareProcessNamespace,omitempty"`
	Subdomain                     string                         `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	SupplementalGroupsPolicy      string                         `json:"supplementalGroupsPolicy,omitempty" yaml:"supplementalGroupsPolicy,omitempty"`
	Sysctls                       []Sysctl                       `json:"sysctls,omitempty" yaml:"sysctls,omitempty"`
	TerminationGracePeriodSeconds *int64                         `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	TopologySpreadConstraints     []TopologySpreadConstraint     `json:"topologySpreadConstraints,omitempty" yaml:"topologySpreadConstraints,omitempty"`
	Uid                           *int64                         `json:"uid,omitempty" yaml:"uid,omitempty"`
	Volumes                       []Volume                       `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WindowsOptions                *WindowsSecurityContextOptions `json:"windowsOptions,omitempty" yaml:"windowsOptions,omitempty"`
}
