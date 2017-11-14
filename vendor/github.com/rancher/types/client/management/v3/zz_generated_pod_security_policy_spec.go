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
	AllowPrivilegeEscalation        *bool                              `json:"allowPrivilegeEscalation,omitempty"`
	AllowedCapabilities             []string                           `json:"allowedCapabilities,omitempty"`
	AllowedHostPaths                []AllowedHostPath                  `json:"allowedHostPaths,omitempty"`
	DefaultAddCapabilities          []string                           `json:"defaultAddCapabilities,omitempty"`
	DefaultAllowPrivilegeEscalation *bool                              `json:"defaultAllowPrivilegeEscalation,omitempty"`
	FSGroup                         *FSGroupStrategyOptions            `json:"fsGroup,omitempty"`
	HostIPC                         *bool                              `json:"hostIPC,omitempty"`
	HostNetwork                     *bool                              `json:"hostNetwork,omitempty"`
	HostPID                         *bool                              `json:"hostPID,omitempty"`
	HostPorts                       []HostPortRange                    `json:"hostPorts,omitempty"`
	Privileged                      *bool                              `json:"privileged,omitempty"`
	ReadOnlyRootFilesystem          *bool                              `json:"readOnlyRootFilesystem,omitempty"`
	RequiredDropCapabilities        []string                           `json:"requiredDropCapabilities,omitempty"`
	RunAsUser                       *RunAsUserStrategyOptions          `json:"runAsUser,omitempty"`
	SELinux                         *SELinuxStrategyOptions            `json:"seLinux,omitempty"`
	SupplementalGroups              *SupplementalGroupsStrategyOptions `json:"supplementalGroups,omitempty"`
	Volumes                         []string                           `json:"volumes,omitempty"`
}
