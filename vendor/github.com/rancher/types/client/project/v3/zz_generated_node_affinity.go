package client

const (
	NodeAffinityType                                                 = "nodeAffinity"
	NodeAffinityFieldPreferredDuringSchedulingIgnoredDuringExecution = "preferredDuringSchedulingIgnoredDuringExecution"
	NodeAffinityFieldRequiredDuringSchedulingIgnoredDuringExecution  = "requiredDuringSchedulingIgnoredDuringExecution"
)

type NodeAffinity struct {
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelector             `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
}
