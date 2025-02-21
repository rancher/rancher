package client

const (
	PodSecurityContextType                          = "podSecurityContext"
	PodSecurityContextFieldAppArmorProfile          = "appArmorProfile"
	PodSecurityContextFieldFSGroupChangePolicy      = "fsGroupChangePolicy"
	PodSecurityContextFieldFsgid                    = "fsgid"
	PodSecurityContextFieldGids                     = "gids"
	PodSecurityContextFieldRunAsGroup               = "runAsGroup"
	PodSecurityContextFieldRunAsNonRoot             = "runAsNonRoot"
	PodSecurityContextFieldSELinuxChangePolicy      = "seLinuxChangePolicy"
	PodSecurityContextFieldSeccompProfile           = "seccompProfile"
	PodSecurityContextFieldSupplementalGroupsPolicy = "supplementalGroupsPolicy"
	PodSecurityContextFieldSysctls                  = "sysctls"
	PodSecurityContextFieldUid                      = "uid"
	PodSecurityContextFieldWindowsOptions           = "windowsOptions"
)

type PodSecurityContext struct {
	AppArmorProfile          *AppArmorProfile               `json:"appArmorProfile,omitempty" yaml:"appArmorProfile,omitempty"`
	FSGroupChangePolicy      string                         `json:"fsGroupChangePolicy,omitempty" yaml:"fsGroupChangePolicy,omitempty"`
	Fsgid                    *int64                         `json:"fsgid,omitempty" yaml:"fsgid,omitempty"`
	Gids                     []int64                        `json:"gids,omitempty" yaml:"gids,omitempty"`
	RunAsGroup               *int64                         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot             *bool                          `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	SELinuxChangePolicy      string                         `json:"seLinuxChangePolicy,omitempty" yaml:"seLinuxChangePolicy,omitempty"`
	SeccompProfile           *SeccompProfile                `json:"seccompProfile,omitempty" yaml:"seccompProfile,omitempty"`
	SupplementalGroupsPolicy string                         `json:"supplementalGroupsPolicy,omitempty" yaml:"supplementalGroupsPolicy,omitempty"`
	Sysctls                  []Sysctl                       `json:"sysctls,omitempty" yaml:"sysctls,omitempty"`
	Uid                      *int64                         `json:"uid,omitempty" yaml:"uid,omitempty"`
	WindowsOptions           *WindowsSecurityContextOptions `json:"windowsOptions,omitempty" yaml:"windowsOptions,omitempty"`
}
