package client

const (
	WeightedPodAffinityTermType                 = "weightedPodAffinityTerm"
	WeightedPodAffinityTermFieldPodAffinityTerm = "podAffinityTerm"
	WeightedPodAffinityTermFieldWeight          = "weight"
)

type WeightedPodAffinityTerm struct {
	PodAffinityTerm *PodAffinityTerm `json:"podAffinityTerm,omitempty"`
	Weight          *int64           `json:"weight,omitempty"`
}
