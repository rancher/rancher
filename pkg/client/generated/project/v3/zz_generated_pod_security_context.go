package client

const (
	PodSecurityContextType                     = "podSecurityContext"
	PodSecurityContextFieldFSGroupChangePolicy = "fsGroupChangePolicy"
	PodSecurityContextFieldFsgid               = "fsgid"
	PodSecurityContextFieldGids                = "gids"
	PodSecurityContextFieldRunAsGroup          = "runAsGroup"
	PodSecurityContextFieldRunAsNonRoot        = "runAsNonRoot"
	PodSecurityContextFieldSysctls             = "sysctls"
	PodSecurityContextFieldUid                 = "uid"
	PodSecurityContextFieldWindowsOptions      = "windowsOptions"
)

type PodSecurityContext struct {
	FSGroupChangePolicy string                         `json:"fsGroupChangePolicy,omitempty" yaml:"fsGroupChangePolicy,omitempty"`
	Fsgid               *int64                         `json:"fsgid,omitempty" yaml:"fsgid,omitempty"`
	Gids                []int64                        `json:"gids,omitempty" yaml:"gids,omitempty"`
	RunAsGroup          *int64                         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot        *bool                          `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	Sysctls             []Sysctl                       `json:"sysctls,omitempty" yaml:"sysctls,omitempty"`
	Uid                 *int64                         `json:"uid,omitempty" yaml:"uid,omitempty"`
	WindowsOptions      *WindowsSecurityContextOptions `json:"windowsOptions,omitempty" yaml:"windowsOptions,omitempty"`
}
