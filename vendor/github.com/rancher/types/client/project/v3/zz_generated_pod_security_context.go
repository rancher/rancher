package client

const (
	PodSecurityContextType              = "podSecurityContext"
	PodSecurityContextFieldFsgid        = "fsgid"
	PodSecurityContextFieldGids         = "gids"
	PodSecurityContextFieldRunAsNonRoot = "runAsNonRoot"
	PodSecurityContextFieldUid          = "uid"
)

type PodSecurityContext struct {
	Fsgid        *int64  `json:"fsgid,omitempty" yaml:"fsgid,omitempty"`
	Gids         []int64 `json:"gids,omitempty" yaml:"gids,omitempty"`
	RunAsNonRoot *bool   `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	Uid          *int64  `json:"uid,omitempty" yaml:"uid,omitempty"`
}
