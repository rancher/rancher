package client

const (
	PodSecurityPolicySpecType                                 = "podSecurityPolicySpec"
	PodSecurityPolicySpecFieldAllowPrivilegeEscalation        = "allowPrivilegeEscalation"
	PodSecurityPolicySpecFieldAllowedCSIDrivers               = "allowedCSIDrivers"
	PodSecurityPolicySpecFieldAllowedCapabilities             = "allowedCapabilities"
	PodSecurityPolicySpecFieldAllowedFlexVolumes              = "allowedFlexVolumes"
	PodSecurityPolicySpecFieldAllowedHostPaths                = "allowedHostPaths"
	PodSecurityPolicySpecFieldAllowedProcMountTypes           = "allowedProcMountTypes"
	PodSecurityPolicySpecFieldAllowedUnsafeSysctls            = "allowedUnsafeSysctls"
	PodSecurityPolicySpecFieldDefaultAddCapabilities          = "defaultAddCapabilities"
	PodSecurityPolicySpecFieldDefaultAllowPrivilegeEscalation = "defaultAllowPrivilegeEscalation"
	PodSecurityPolicySpecFieldFSGroup                         = "fsGroup"
	PodSecurityPolicySpecFieldForbiddenSysctls                = "forbiddenSysctls"
	PodSecurityPolicySpecFieldHostIPC                         = "hostIPC"
	PodSecurityPolicySpecFieldHostNetwork                     = "hostNetwork"
	PodSecurityPolicySpecFieldHostPID                         = "hostPID"
	PodSecurityPolicySpecFieldHostPorts                       = "hostPorts"
	PodSecurityPolicySpecFieldPrivileged                      = "privileged"
	PodSecurityPolicySpecFieldReadOnlyRootFilesystem          = "readOnlyRootFilesystem"
	PodSecurityPolicySpecFieldRequiredDropCapabilities        = "requiredDropCapabilities"
	PodSecurityPolicySpecFieldRunAsGroup                      = "runAsGroup"
	PodSecurityPolicySpecFieldRunAsUser                       = "runAsUser"
	PodSecurityPolicySpecFieldRuntimeClass                    = "runtimeClass"
	PodSecurityPolicySpecFieldSELinux                         = "seLinux"
	PodSecurityPolicySpecFieldSupplementalGroups              = "supplementalGroups"
	PodSecurityPolicySpecFieldVolumes                         = "volumes"
)

type PodSecurityPolicySpec struct {
	AllowPrivilegeEscalation        *bool                              `json:"allowPrivilegeEscalation,omitempty" yaml:"allowPrivilegeEscalation,omitempty"`
	AllowedCSIDrivers               []AllowedCSIDriver                 `json:"allowedCSIDrivers,omitempty" yaml:"allowedCSIDrivers,omitempty"`
	AllowedCapabilities             []string                           `json:"allowedCapabilities,omitempty" yaml:"allowedCapabilities,omitempty"`
	AllowedFlexVolumes              []AllowedFlexVolume                `json:"allowedFlexVolumes,omitempty" yaml:"allowedFlexVolumes,omitempty"`
	AllowedHostPaths                []AllowedHostPath                  `json:"allowedHostPaths,omitempty" yaml:"allowedHostPaths,omitempty"`
	AllowedProcMountTypes           []string                           `json:"allowedProcMountTypes,omitempty" yaml:"allowedProcMountTypes,omitempty"`
	AllowedUnsafeSysctls            []string                           `json:"allowedUnsafeSysctls,omitempty" yaml:"allowedUnsafeSysctls,omitempty"`
	DefaultAddCapabilities          []string                           `json:"defaultAddCapabilities,omitempty" yaml:"defaultAddCapabilities,omitempty"`
	DefaultAllowPrivilegeEscalation *bool                              `json:"defaultAllowPrivilegeEscalation,omitempty" yaml:"defaultAllowPrivilegeEscalation,omitempty"`
	FSGroup                         *FSGroupStrategyOptions            `json:"fsGroup,omitempty" yaml:"fsGroup,omitempty"`
	ForbiddenSysctls                []string                           `json:"forbiddenSysctls,omitempty" yaml:"forbiddenSysctls,omitempty"`
	HostIPC                         bool                               `json:"hostIPC,omitempty" yaml:"hostIPC,omitempty"`
	HostNetwork                     bool                               `json:"hostNetwork,omitempty" yaml:"hostNetwork,omitempty"`
	HostPID                         bool                               `json:"hostPID,omitempty" yaml:"hostPID,omitempty"`
	HostPorts                       []HostPortRange                    `json:"hostPorts,omitempty" yaml:"hostPorts,omitempty"`
	Privileged                      bool                               `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	ReadOnlyRootFilesystem          bool                               `json:"readOnlyRootFilesystem,omitempty" yaml:"readOnlyRootFilesystem,omitempty"`
	RequiredDropCapabilities        []string                           `json:"requiredDropCapabilities,omitempty" yaml:"requiredDropCapabilities,omitempty"`
	RunAsGroup                      *RunAsGroupStrategyOptions         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsUser                       *RunAsUserStrategyOptions          `json:"runAsUser,omitempty" yaml:"runAsUser,omitempty"`
	RuntimeClass                    *RuntimeClassStrategyOptions       `json:"runtimeClass,omitempty" yaml:"runtimeClass,omitempty"`
	SELinux                         *SELinuxStrategyOptions            `json:"seLinux,omitempty" yaml:"seLinux,omitempty"`
	SupplementalGroups              *SupplementalGroupsStrategyOptions `json:"supplementalGroups,omitempty" yaml:"supplementalGroups,omitempty"`
	Volumes                         []string                           `json:"volumes,omitempty" yaml:"volumes,omitempty"`
}
