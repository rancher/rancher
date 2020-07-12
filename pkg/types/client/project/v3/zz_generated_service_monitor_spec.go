package client

const (
	ServiceMonitorSpecType                   = "serviceMonitorSpec"
	ServiceMonitorSpecFieldEndpoints         = "endpoints"
	ServiceMonitorSpecFieldJobLabel          = "jobLabel"
	ServiceMonitorSpecFieldNamespaceSelector = "namespaceSelector"
	ServiceMonitorSpecFieldPodTargetLabels   = "podTargetLabels"
	ServiceMonitorSpecFieldSampleLimit       = "sampleLimit"
	ServiceMonitorSpecFieldSelector          = "selector"
	ServiceMonitorSpecFieldTargetLabels      = "targetLabels"
)

type ServiceMonitorSpec struct {
	Endpoints         []Endpoint     `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	JobLabel          string         `json:"jobLabel,omitempty" yaml:"jobLabel,omitempty"`
	NamespaceSelector []string       `json:"namespaceSelector,omitempty" yaml:"namespaceSelector,omitempty"`
	PodTargetLabels   []string       `json:"podTargetLabels,omitempty" yaml:"podTargetLabels,omitempty"`
	SampleLimit       int64          `json:"sampleLimit,omitempty" yaml:"sampleLimit,omitempty"`
	Selector          *LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`
	TargetLabels      []string       `json:"targetLabels,omitempty" yaml:"targetLabels,omitempty"`
}
