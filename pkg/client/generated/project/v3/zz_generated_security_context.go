package client

const (
	SecurityContextType                          = "securityContext"
	SecurityContextFieldAllowPrivilegeEscalation = "allowPrivilegeEscalation"
	SecurityContextFieldCapAdd                   = "capAdd"
	SecurityContextFieldCapDrop                  = "capDrop"
	SecurityContextFieldPrivileged               = "privileged"
	SecurityContextFieldProcMount                = "procMount"
	SecurityContextFieldReadOnly                 = "readOnly"
	SecurityContextFieldRunAsGroup               = "runAsGroup"
	SecurityContextFieldRunAsNonRoot             = "runAsNonRoot"
	SecurityContextFieldSeccompProfile           = "seccompProfile"
	SecurityContextFieldUid                      = "uid"
	SecurityContextFieldWindowsOptions           = "windowsOptions"
)

type SecurityContext struct {
	AllowPrivilegeEscalation *bool                          `json:"allowPrivilegeEscalation,omitempty" yaml:"allowPrivilegeEscalation,omitempty"`
	CapAdd                   []string                       `json:"capAdd,omitempty" yaml:"capAdd,omitempty"`
	CapDrop                  []string                       `json:"capDrop,omitempty" yaml:"capDrop,omitempty"`
	Privileged               *bool                          `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	ProcMount                string                         `json:"procMount,omitempty" yaml:"procMount,omitempty"`
	ReadOnly                 *bool                          `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	RunAsGroup               *int64                         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot             *bool                          `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	SeccompProfile           *SeccompProfile                `json:"seccompProfile,omitempty" yaml:"seccompProfile,omitempty"`
	Uid                      *int64                         `json:"uid,omitempty" yaml:"uid,omitempty"`
	WindowsOptions           *WindowsSecurityContextOptions `json:"windowsOptions,omitempty" yaml:"windowsOptions,omitempty"`
}
