package client

const (
	ProjectAlertRuleSpecType                       = "projectAlertRuleSpec"
	ProjectAlertRuleSpecFieldDisplayName           = "displayName"
	ProjectAlertRuleSpecFieldGroupID               = "groupId"
	ProjectAlertRuleSpecFieldGroupIntervalSeconds  = "groupIntervalSeconds"
	ProjectAlertRuleSpecFieldGroupWaitSeconds      = "groupWaitSeconds"
	ProjectAlertRuleSpecFieldInherited             = "inherited"
	ProjectAlertRuleSpecFieldMetricRule            = "metricRule"
	ProjectAlertRuleSpecFieldPodRule               = "podRule"
	ProjectAlertRuleSpecFieldProjectID             = "projectId"
	ProjectAlertRuleSpecFieldRepeatIntervalSeconds = "repeatIntervalSeconds"
	ProjectAlertRuleSpecFieldSeverity              = "severity"
	ProjectAlertRuleSpecFieldWorkloadRule          = "workloadRule"
)

type ProjectAlertRuleSpec struct {
	DisplayName           string        `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	GroupID               string        `json:"groupId,omitempty" yaml:"groupId,omitempty"`
	GroupIntervalSeconds  int64         `json:"groupIntervalSeconds,omitempty" yaml:"groupIntervalSeconds,omitempty"`
	GroupWaitSeconds      int64         `json:"groupWaitSeconds,omitempty" yaml:"groupWaitSeconds,omitempty"`
	Inherited             *bool         `json:"inherited,omitempty" yaml:"inherited,omitempty"`
	MetricRule            *MetricRule   `json:"metricRule,omitempty" yaml:"metricRule,omitempty"`
	PodRule               *PodRule      `json:"podRule,omitempty" yaml:"podRule,omitempty"`
	ProjectID             string        `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	RepeatIntervalSeconds int64         `json:"repeatIntervalSeconds,omitempty" yaml:"repeatIntervalSeconds,omitempty"`
	Severity              string        `json:"severity,omitempty" yaml:"severity,omitempty"`
	WorkloadRule          *WorkloadRule `json:"workloadRule,omitempty" yaml:"workloadRule,omitempty"`
}
