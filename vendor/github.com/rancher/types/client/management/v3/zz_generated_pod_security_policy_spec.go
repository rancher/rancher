package client

const (
	PodSecurityPolicySpecType                                 = "podSecurityPolicySpec"
	PodSecurityPolicySpecFieldAllowPrivilegeEscalation        = "allowPrivilegeEscalation"
	PodSecurityPolicySpecFieldAllowedCapabilities             = "allowedCapabilities"
	PodSecurityPolicySpecFieldAllowedHostPaths                = "allowedHostPaths"
	PodSecurityPolicySpecFieldDefaultAddCapabilities          = "defaultAddCapabilities"
	PodSecurityPolicySpecFieldDefaultAllowPrivilegeEscalation = "defaultAllowPrivilegeEscalation"
	PodSecurityPolicySpecFieldFSGroup                         = "fsGroup"
	PodSecurityPolicySpecFieldHostIPC                         = "hostIPC"
	PodSecurityPolicySpecFieldHostNetwork                     = "hostNetwork"
	PodSecurityPolicySpecFieldHostPID                         = "hostPID"
	PodSecurityPolicySpecFieldHostPorts                       = "hostPorts"
	PodSecurityPolicySpecFieldPrivileged                      = "privileged"
	PodSecurityPolicySpecFieldReadOnlyRootFilesystem          = "readOnlyRootFilesystem"
	PodSecurityPolicySpecFieldRequiredDropCapabilities        = "requiredDropCapabilities"
	PodSecurityPolicySpecFieldRunAsUser                       = "runAsUser"
	PodSecurityPolicySpecFieldSELinux                         = "seLinux"
	PodSecurityPolicySpecFieldSupplementalGroups              = "supplementalGroups"
	PodSecurityPolicySpecFieldVolumes                         = "volumes"
)

type PodSecurityPolicySpec struct {
	AllowPrivilegeEscalation        *bool                              `json:"allowPrivilegeEscalation,omitempty" yaml:"allowPrivilegeEscalation,omitempty"`
	AllowedCapabilities             []string                           `json:"allowedCapabilities,omitempty" yaml:"allowedCapabilities,omitempty"`
	AllowedHostPaths                []AllowedHostPath                  `json:"allowedHostPaths,omitempty" yaml:"allowedHostPaths,omitempty"`
	DefaultAddCapabilities          []string                           `json:"defaultAddCapabilities,omitempty" yaml:"defaultAddCapabilities,omitempty"`
	DefaultAllowPrivilegeEscalation *bool                              `json:"defaultAllowPrivilegeEscalation,omitempty" yaml:"defaultAllowPrivilegeEscalation,omitempty"`
	FSGroup                         *FSGroupStrategyOptions            `json:"fsGroup,omitempty" yaml:"fsGroup,omitempty"`
	HostIPC                         bool                               `json:"hostIPC,omitempty" yaml:"hostIPC,omitempty"`
	HostNetwork                     bool                               `json:"hostNetwork,omitempty" yaml:"hostNetwork,omitempty"`
	HostPID                         bool                               `json:"hostPID,omitempty" yaml:"hostPID,omitempty"`
	HostPorts                       []HostPortRange                    `json:"hostPorts,omitempty" yaml:"hostPorts,omitempty"`
	Privileged                      bool                               `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	ReadOnlyRootFilesystem          bool                               `json:"readOnlyRootFilesystem,omitempty" yaml:"readOnlyRootFilesystem,omitempty"`
	RequiredDropCapabilities        []string                           `json:"requiredDropCapabilities,omitempty" yaml:"requiredDropCapabilities,omitempty"`
	RunAsUser                       *RunAsUserStrategyOptions          `json:"runAsUser,omitempty" yaml:"runAsUser,omitempty"`
	SELinux                         *SELinuxStrategyOptions            `json:"seLinux,omitempty" yaml:"seLinux,omitempty"`
	SupplementalGroups              *SupplementalGroupsStrategyOptions `json:"supplementalGroups,omitempty" yaml:"supplementalGroups,omitempty"`
	Volumes                         []string                           `json:"volumes,omitempty" yaml:"volumes,omitempty"`
}
