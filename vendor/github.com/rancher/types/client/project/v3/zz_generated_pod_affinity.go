package client

const (
	PodAffinityType                                                 = "podAffinity"
	PodAffinityFieldPreferredDuringSchedulingIgnoredDuringExecution = "preferredDuringSchedulingIgnoredDuringExecution"
	PodAffinityFieldRequiredDuringSchedulingIgnoredDuringExecution  = "requiredDuringSchedulingIgnoredDuringExecution"
)

type PodAffinity struct {
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
}
