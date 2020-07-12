package client

const (
	HorizontalPodAutoscalerStatusType                    = "horizontalPodAutoscalerStatus"
	HorizontalPodAutoscalerStatusFieldConditions         = "conditions"
	HorizontalPodAutoscalerStatusFieldCurrentMetrics     = "currentMetrics"
	HorizontalPodAutoscalerStatusFieldCurrentReplicas    = "currentReplicas"
	HorizontalPodAutoscalerStatusFieldDesiredReplicas    = "desiredReplicas"
	HorizontalPodAutoscalerStatusFieldLastScaleTime      = "lastScaleTime"
	HorizontalPodAutoscalerStatusFieldObservedGeneration = "observedGeneration"
)

type HorizontalPodAutoscalerStatus struct {
	Conditions         []HorizontalPodAutoscalerCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	CurrentMetrics     []MetricStatus                     `json:"currentMetrics,omitempty" yaml:"currentMetrics,omitempty"`
	CurrentReplicas    int64                              `json:"currentReplicas,omitempty" yaml:"currentReplicas,omitempty"`
	DesiredReplicas    int64                              `json:"desiredReplicas,omitempty" yaml:"desiredReplicas,omitempty"`
	LastScaleTime      string                             `json:"lastScaleTime,omitempty" yaml:"lastScaleTime,omitempty"`
	ObservedGeneration *int64                             `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
}
