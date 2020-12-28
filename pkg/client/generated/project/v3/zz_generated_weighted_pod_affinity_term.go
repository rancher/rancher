package client

const (
	WeightedPodAffinityTermType                 = "weightedPodAffinityTerm"
	WeightedPodAffinityTermFieldPodAffinityTerm = "podAffinityTerm"
	WeightedPodAffinityTermFieldWeight          = "weight"
)

type WeightedPodAffinityTerm struct {
	PodAffinityTerm *PodAffinityTerm `json:"podAffinityTerm,omitempty" yaml:"podAffinityTerm,omitempty"`
	Weight          int64            `json:"weight,omitempty" yaml:"weight,omitempty"`
}
