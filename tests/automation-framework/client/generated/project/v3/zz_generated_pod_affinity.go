package client

const (
	PodAffinityType                                                 = "podAffinity"
	PodAffinityFieldPreferredDuringSchedulingIgnoredDuringExecution = "preferredDuringSchedulingIgnoredDuringExecution"
	PodAffinityFieldRequiredDuringSchedulingIgnoredDuringExecution  = "requiredDuringSchedulingIgnoredDuringExecution"
)

type PodAffinity struct {
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty" yaml:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty" yaml:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
}
