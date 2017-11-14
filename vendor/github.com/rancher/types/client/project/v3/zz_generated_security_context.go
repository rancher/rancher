package client

const (
	SecurityContextType                          = "securityContext"
	SecurityContextFieldAllowPrivilegeEscalation = "allowPrivilegeEscalation"
	SecurityContextFieldCapAdd                   = "capAdd"
	SecurityContextFieldCapDrop                  = "capDrop"
	SecurityContextFieldPrivileged               = "privileged"
	SecurityContextFieldReadOnly                 = "readOnly"
	SecurityContextFieldRunAsNonRoot             = "runAsNonRoot"
	SecurityContextFieldUid                      = "uid"
)

type SecurityContext struct {
	AllowPrivilegeEscalation *bool    `json:"allowPrivilegeEscalation,omitempty"`
	CapAdd                   []string `json:"capAdd,omitempty"`
	CapDrop                  []string `json:"capDrop,omitempty"`
	Privileged               *bool    `json:"privileged,omitempty"`
	ReadOnly                 *bool    `json:"readOnly,omitempty"`
	RunAsNonRoot             *bool    `json:"runAsNonRoot,omitempty"`
	Uid                      *int64   `json:"uid,omitempty"`
}
