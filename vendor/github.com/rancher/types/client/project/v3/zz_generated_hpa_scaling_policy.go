package client

const (
	HPAScalingPolicyType               = "hpaScalingPolicy"
	HPAScalingPolicyFieldPeriodSeconds = "periodSeconds"
	HPAScalingPolicyFieldType          = "type"
	HPAScalingPolicyFieldValue         = "value"
)

type HPAScalingPolicy struct {
	PeriodSeconds int64  `json:"periodSeconds,omitempty" yaml:"periodSeconds,omitempty"`
	Type          string `json:"type,omitempty" yaml:"type,omitempty"`
	Value         int64  `json:"value,omitempty" yaml:"value,omitempty"`
}
