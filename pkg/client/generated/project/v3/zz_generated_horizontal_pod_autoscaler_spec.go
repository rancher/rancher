package client

const (
	HorizontalPodAutoscalerSpecType                = "horizontalPodAutoscalerSpec"
	HorizontalPodAutoscalerSpecFieldBehavior       = "behavior"
	HorizontalPodAutoscalerSpecFieldMaxReplicas    = "maxReplicas"
	HorizontalPodAutoscalerSpecFieldMetrics        = "metrics"
	HorizontalPodAutoscalerSpecFieldMinReplicas    = "minReplicas"
	HorizontalPodAutoscalerSpecFieldScaleTargetRef = "scaleTargetRef"
)

type HorizontalPodAutoscalerSpec struct {
	Behavior       *HorizontalPodAutoscalerBehavior `json:"behavior,omitempty" yaml:"behavior,omitempty"`
	MaxReplicas    int64                            `json:"maxReplicas,omitempty" yaml:"maxReplicas,omitempty"`
	Metrics        []Metric                         `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	MinReplicas    *int64                           `json:"minReplicas,omitempty" yaml:"minReplicas,omitempty"`
	ScaleTargetRef *CrossVersionObjectReference     `json:"scaleTargetRef,omitempty" yaml:"scaleTargetRef,omitempty"`
}
