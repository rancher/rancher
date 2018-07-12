package client

const (
	SecurityContextType                          = "securityContext"
	SecurityContextFieldAllowPrivilegeEscalation = "allowPrivilegeEscalation"
	SecurityContextFieldCapAdd                   = "capAdd"
	SecurityContextFieldCapDrop                  = "capDrop"
	SecurityContextFieldPrivileged               = "privileged"
	SecurityContextFieldReadOnly                 = "readOnly"
	SecurityContextFieldRunAsGroup               = "runAsGroup"
	SecurityContextFieldRunAsNonRoot             = "runAsNonRoot"
	SecurityContextFieldUid                      = "uid"
)

type SecurityContext struct {
	AllowPrivilegeEscalation *bool    `json:"allowPrivilegeEscalation,omitempty" yaml:"allowPrivilegeEscalation,omitempty"`
	CapAdd                   []string `json:"capAdd,omitempty" yaml:"capAdd,omitempty"`
	CapDrop                  []string `json:"capDrop,omitempty" yaml:"capDrop,omitempty"`
	Privileged               *bool    `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	ReadOnly                 *bool    `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	RunAsGroup               *int64   `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot             *bool    `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	Uid                      *int64   `json:"uid,omitempty" yaml:"uid,omitempty"`
}
