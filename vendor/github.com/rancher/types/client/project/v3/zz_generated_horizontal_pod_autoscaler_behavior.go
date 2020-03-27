package client

const (
	HorizontalPodAutoscalerBehaviorType           = "horizontalPodAutoscalerBehavior"
	HorizontalPodAutoscalerBehaviorFieldScaleDown = "scaleDown"
	HorizontalPodAutoscalerBehaviorFieldScaleUp   = "scaleUp"
)

type HorizontalPodAutoscalerBehavior struct {
	ScaleDown *HPAScalingRules `json:"scaleDown,omitempty" yaml:"scaleDown,omitempty"`
	ScaleUp   *HPAScalingRules `json:"scaleUp,omitempty" yaml:"scaleUp,omitempty"`
}
