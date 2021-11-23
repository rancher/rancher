package client

const (
	ServiceMonitorSpecType                       = "serviceMonitorSpec"
	ServiceMonitorSpecFieldEndpoints             = "endpoints"
	ServiceMonitorSpecFieldJobLabel              = "jobLabel"
	ServiceMonitorSpecFieldLabelLimit            = "labelLimit"
	ServiceMonitorSpecFieldLabelNameLengthLimit  = "labelNameLengthLimit"
	ServiceMonitorSpecFieldLabelValueLengthLimit = "labelValueLengthLimit"
	ServiceMonitorSpecFieldNamespaceSelector     = "namespaceSelector"
	ServiceMonitorSpecFieldPodTargetLabels       = "podTargetLabels"
	ServiceMonitorSpecFieldSampleLimit           = "sampleLimit"
	ServiceMonitorSpecFieldSelector              = "selector"
	ServiceMonitorSpecFieldTargetLabels          = "targetLabels"
	ServiceMonitorSpecFieldTargetLimit           = "targetLimit"
)

type ServiceMonitorSpec struct {
	Endpoints             []Endpoint     `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	JobLabel              string         `json:"jobLabel,omitempty" yaml:"jobLabel,omitempty"`
	LabelLimit            int64          `json:"labelLimit,omitempty" yaml:"labelLimit,omitempty"`
	LabelNameLengthLimit  int64          `json:"labelNameLengthLimit,omitempty" yaml:"labelNameLengthLimit,omitempty"`
	LabelValueLengthLimit int64          `json:"labelValueLengthLimit,omitempty" yaml:"labelValueLengthLimit,omitempty"`
	NamespaceSelector     []string       `json:"namespaceSelector,omitempty" yaml:"namespaceSelector,omitempty"`
	PodTargetLabels       []string       `json:"podTargetLabels,omitempty" yaml:"podTargetLabels,omitempty"`
	SampleLimit           int64          `json:"sampleLimit,omitempty" yaml:"sampleLimit,omitempty"`
	Selector              *LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`
	TargetLabels          []string       `json:"targetLabels,omitempty" yaml:"targetLabels,omitempty"`
	TargetLimit           int64          `json:"targetLimit,omitempty" yaml:"targetLimit,omitempty"`
}
