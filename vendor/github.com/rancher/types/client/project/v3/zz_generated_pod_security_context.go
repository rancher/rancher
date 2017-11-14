package client

const (
	PodSecurityContextType              = "podSecurityContext"
	PodSecurityContextFieldFsgid        = "fsgid"
	PodSecurityContextFieldGids         = "gids"
	PodSecurityContextFieldRunAsNonRoot = "runAsNonRoot"
	PodSecurityContextFieldUid          = "uid"
)

type PodSecurityContext struct {
	Fsgid        *int64  `json:"fsgid,omitempty"`
	Gids         []int64 `json:"gids,omitempty"`
	RunAsNonRoot *bool   `json:"runAsNonRoot,omitempty"`
	Uid          *int64  `json:"uid,omitempty"`
}
