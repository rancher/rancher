package client

const (
	ProjectMonitorGraphSpecType                        = "projectMonitorGraphSpec"
	ProjectMonitorGraphSpecFieldDescription            = "description"
	ProjectMonitorGraphSpecFieldDetailsMetricsSelector = "detailsMetricsSelector"
	ProjectMonitorGraphSpecFieldDisplayResourceType    = "displayResourceType"
	ProjectMonitorGraphSpecFieldGraphType              = "graphType"
	ProjectMonitorGraphSpecFieldMetricsSelector        = "metricsSelector"
	ProjectMonitorGraphSpecFieldPriority               = "priority"
	ProjectMonitorGraphSpecFieldProjectID              = "projectId"
	ProjectMonitorGraphSpecFieldResourceType           = "resourceType"
	ProjectMonitorGraphSpecFieldYAxis                  = "yAxis"
)

type ProjectMonitorGraphSpec struct {
	Description            string            `json:"description,omitempty" yaml:"description,omitempty"`
	DetailsMetricsSelector map[string]string `json:"detailsMetricsSelector,omitempty" yaml:"detailsMetricsSelector,omitempty"`
	DisplayResourceType    string            `json:"displayResourceType,omitempty" yaml:"displayResourceType,omitempty"`
	GraphType              string            `json:"graphType,omitempty" yaml:"graphType,omitempty"`
	MetricsSelector        map[string]string `json:"metricsSelector,omitempty" yaml:"metricsSelector,omitempty"`
	Priority               int64             `json:"priority,omitempty" yaml:"priority,omitempty"`
	ProjectID              string            `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	ResourceType           string            `json:"resourceType,omitempty" yaml:"resourceType,omitempty"`
	YAxis                  *YAxis            `json:"yAxis,omitempty" yaml:"yAxis,omitempty"`
}
