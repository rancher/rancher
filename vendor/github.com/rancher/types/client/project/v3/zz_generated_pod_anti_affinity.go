package client

const (
	PodAntiAffinityType                                                 = "podAntiAffinity"
	PodAntiAffinityFieldPreferredDuringSchedulingIgnoredDuringExecution = "preferredDuringSchedulingIgnoredDuringExecution"
	PodAntiAffinityFieldRequiredDuringSchedulingIgnoredDuringExecution  = "requiredDuringSchedulingIgnoredDuringExecution"
)

type PodAntiAffinity struct {
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
}
