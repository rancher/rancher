package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	StatefulSetSpecType                                      = "statefulSetSpec"
	StatefulSetSpecFieldActiveDeadlineSeconds                = "activeDeadlineSeconds"
	StatefulSetSpecFieldAppArmorProfile                      = "appArmorProfile"
	StatefulSetSpecFieldAutomountServiceAccountToken         = "automountServiceAccountToken"
	StatefulSetSpecFieldContainers                           = "containers"
	StatefulSetSpecFieldDNSConfig                            = "dnsConfig"
	StatefulSetSpecFieldDNSPolicy                            = "dnsPolicy"
	StatefulSetSpecFieldEnableServiceLinks                   = "enableServiceLinks"
	StatefulSetSpecFieldEphemeralContainers                  = "ephemeralContainers"
	StatefulSetSpecFieldFSGroupChangePolicy                  = "fsGroupChangePolicy"
	StatefulSetSpecFieldFsgid                                = "fsgid"
	StatefulSetSpecFieldGids                                 = "gids"
	StatefulSetSpecFieldHostAliases                          = "hostAliases"
	StatefulSetSpecFieldHostIPC                              = "hostIPC"
	StatefulSetSpecFieldHostNetwork                          = "hostNetwork"
	StatefulSetSpecFieldHostPID                              = "hostPID"
	StatefulSetSpecFieldHostUsers                            = "hostUsers"
	StatefulSetSpecFieldHostname                             = "hostname"
	StatefulSetSpecFieldImagePullSecrets                     = "imagePullSecrets"
	StatefulSetSpecFieldMaxUnavailable                       = "maxUnavailable"
	StatefulSetSpecFieldMinReadySeconds                      = "minReadySeconds"
	StatefulSetSpecFieldNodeID                               = "nodeId"
	StatefulSetSpecFieldOS                                   = "os"
	StatefulSetSpecFieldObjectMeta                           = "metadata"
	StatefulSetSpecFieldOrdinals                             = "ordinals"
	StatefulSetSpecFieldOverhead                             = "overhead"
	StatefulSetSpecFieldPersistentVolumeClaimRetentionPolicy = "persistentVolumeClaimRetentionPolicy"
	StatefulSetSpecFieldPreemptionPolicy                     = "preemptionPolicy"
	StatefulSetSpecFieldReadinessGates                       = "readinessGates"
	StatefulSetSpecFieldResourceClaims                       = "resourceClaims"
	StatefulSetSpecFieldRestartPolicy                        = "restartPolicy"
	StatefulSetSpecFieldRunAsGroup                           = "runAsGroup"
	StatefulSetSpecFieldRunAsNonRoot                         = "runAsNonRoot"
	StatefulSetSpecFieldRuntimeClassName                     = "runtimeClassName"
	StatefulSetSpecFieldScale                                = "scale"
	StatefulSetSpecFieldScheduling                           = "scheduling"
	StatefulSetSpecFieldSchedulingGates                      = "schedulingGates"
	StatefulSetSpecFieldSeccompProfile                       = "seccompProfile"
	StatefulSetSpecFieldSelector                             = "selector"
	StatefulSetSpecFieldServiceAccountName                   = "serviceAccountName"
	StatefulSetSpecFieldSetHostnameAsFQDN                    = "setHostnameAsFQDN"
	StatefulSetSpecFieldShareProcessNamespace                = "shareProcessNamespace"
	StatefulSetSpecFieldStatefulSetConfig                    = "statefulSetConfig"
	StatefulSetSpecFieldSubdomain                            = "subdomain"
	StatefulSetSpecFieldSysctls                              = "sysctls"
	StatefulSetSpecFieldTerminationGracePeriodSeconds        = "terminationGracePeriodSeconds"
	StatefulSetSpecFieldTopologySpreadConstraints            = "topologySpreadConstraints"
	StatefulSetSpecFieldUid                                  = "uid"
	StatefulSetSpecFieldVolumes                              = "volumes"
	StatefulSetSpecFieldWindowsOptions                       = "windowsOptions"
)

type StatefulSetSpec struct {
	ActiveDeadlineSeconds                *int64                                           `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	AppArmorProfile                      *AppArmorProfile                                 `json:"appArmorProfile,omitempty" yaml:"appArmorProfile,omitempty"`
	AutomountServiceAccountToken         *bool                                            `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	Containers                           []Container                                      `json:"containers,omitempty" yaml:"containers,omitempty"`
	DNSConfig                            *PodDNSConfig                                    `json:"dnsConfig,omitempty" yaml:"dnsConfig,omitempty"`
	DNSPolicy                            string                                           `json:"dnsPolicy,omitempty" yaml:"dnsPolicy,omitempty"`
	EnableServiceLinks                   *bool                                            `json:"enableServiceLinks,omitempty" yaml:"enableServiceLinks,omitempty"`
	EphemeralContainers                  []EphemeralContainer                             `json:"ephemeralContainers,omitempty" yaml:"ephemeralContainers,omitempty"`
	FSGroupChangePolicy                  string                                           `json:"fsGroupChangePolicy,omitempty" yaml:"fsGroupChangePolicy,omitempty"`
	Fsgid                                *int64                                           `json:"fsgid,omitempty" yaml:"fsgid,omitempty"`
	Gids                                 []int64                                          `json:"gids,omitempty" yaml:"gids,omitempty"`
	HostAliases                          []HostAlias                                      `json:"hostAliases,omitempty" yaml:"hostAliases,omitempty"`
	HostIPC                              bool                                             `json:"hostIPC,omitempty" yaml:"hostIPC,omitempty"`
	HostNetwork                          bool                                             `json:"hostNetwork,omitempty" yaml:"hostNetwork,omitempty"`
	HostPID                              bool                                             `json:"hostPID,omitempty" yaml:"hostPID,omitempty"`
	HostUsers                            *bool                                            `json:"hostUsers,omitempty" yaml:"hostUsers,omitempty"`
	Hostname                             string                                           `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	ImagePullSecrets                     []LocalObjectReference                           `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	MaxUnavailable                       intstr.IntOrString                               `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
	MinReadySeconds                      int64                                            `json:"minReadySeconds,omitempty" yaml:"minReadySeconds,omitempty"`
	NodeID                               string                                           `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	OS                                   *PodOS                                           `json:"os,omitempty" yaml:"os,omitempty"`
	ObjectMeta                           *ObjectMeta                                      `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Ordinals                             *StatefulSetOrdinals                             `json:"ordinals,omitempty" yaml:"ordinals,omitempty"`
	Overhead                             map[string]string                                `json:"overhead,omitempty" yaml:"overhead,omitempty"`
	PersistentVolumeClaimRetentionPolicy *StatefulSetPersistentVolumeClaimRetentionPolicy `json:"persistentVolumeClaimRetentionPolicy,omitempty" yaml:"persistentVolumeClaimRetentionPolicy,omitempty"`
	PreemptionPolicy                     string                                           `json:"preemptionPolicy,omitempty" yaml:"preemptionPolicy,omitempty"`
	ReadinessGates                       []PodReadinessGate                               `json:"readinessGates,omitempty" yaml:"readinessGates,omitempty"`
	ResourceClaims                       []PodResourceClaim                               `json:"resourceClaims,omitempty" yaml:"resourceClaims,omitempty"`
	RestartPolicy                        string                                           `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsGroup                           *int64                                           `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot                         *bool                                            `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	RuntimeClassName                     string                                           `json:"runtimeClassName,omitempty" yaml:"runtimeClassName,omitempty"`
	Scale                                *int64                                           `json:"scale,omitempty" yaml:"scale,omitempty"`
	Scheduling                           *Scheduling                                      `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	SchedulingGates                      []PodSchedulingGate                              `json:"schedulingGates,omitempty" yaml:"schedulingGates,omitempty"`
	SeccompProfile                       *SeccompProfile                                  `json:"seccompProfile,omitempty" yaml:"seccompProfile,omitempty"`
	Selector                             *LabelSelector                                   `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceAccountName                   string                                           `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	SetHostnameAsFQDN                    *bool                                            `json:"setHostnameAsFQDN,omitempty" yaml:"setHostnameAsFQDN,omitempty"`
	ShareProcessNamespace                *bool                                            `json:"shareProcessNamespace,omitempty" yaml:"shareProcessNamespace,omitempty"`
	StatefulSetConfig                    *StatefulSetConfig                               `json:"statefulSetConfig,omitempty" yaml:"statefulSetConfig,omitempty"`
	Subdomain                            string                                           `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	Sysctls                              []Sysctl                                         `json:"sysctls,omitempty" yaml:"sysctls,omitempty"`
	TerminationGracePeriodSeconds        *int64                                           `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	TopologySpreadConstraints            []TopologySpreadConstraint                       `json:"topologySpreadConstraints,omitempty" yaml:"topologySpreadConstraints,omitempty"`
	Uid                                  *int64                                           `json:"uid,omitempty" yaml:"uid,omitempty"`
	Volumes                              []Volume                                         `json:"volumes,omitempty" yaml:"volumes,omitempty"`
	WindowsOptions                       *WindowsSecurityContextOptions                   `json:"windowsOptions,omitempty" yaml:"windowsOptions,omitempty"`
}
