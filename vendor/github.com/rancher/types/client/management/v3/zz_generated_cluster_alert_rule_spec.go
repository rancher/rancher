package client

const (
	ClusterAlertRuleSpecType                       = "clusterAlertRuleSpec"
	ClusterAlertRuleSpecFieldClusterID             = "clusterId"
	ClusterAlertRuleSpecFieldDisplayName           = "displayName"
	ClusterAlertRuleSpecFieldEventRule             = "eventRule"
	ClusterAlertRuleSpecFieldGroupID               = "groupId"
	ClusterAlertRuleSpecFieldGroupIntervalSeconds  = "groupIntervalSeconds"
	ClusterAlertRuleSpecFieldGroupWaitSeconds      = "groupWaitSeconds"
	ClusterAlertRuleSpecFieldMetricRule            = "metricRule"
	ClusterAlertRuleSpecFieldNodeRule              = "nodeRule"
	ClusterAlertRuleSpecFieldRepeatIntervalSeconds = "repeatIntervalSeconds"
	ClusterAlertRuleSpecFieldSeverity              = "severity"
	ClusterAlertRuleSpecFieldSystemServiceRule     = "systemServiceRule"
)

type ClusterAlertRuleSpec struct {
	ClusterID             string             `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	DisplayName           string             `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	EventRule             *EventRule         `json:"eventRule,omitempty" yaml:"eventRule,omitempty"`
	GroupID               string             `json:"groupId,omitempty" yaml:"groupId,omitempty"`
	GroupIntervalSeconds  int64              `json:"groupIntervalSeconds,omitempty" yaml:"groupIntervalSeconds,omitempty"`
	GroupWaitSeconds      int64              `json:"groupWaitSeconds,omitempty" yaml:"groupWaitSeconds,omitempty"`
	MetricRule            *MetricRule        `json:"metricRule,omitempty" yaml:"metricRule,omitempty"`
	NodeRule              *NodeRule          `json:"nodeRule,omitempty" yaml:"nodeRule,omitempty"`
	RepeatIntervalSeconds int64              `json:"repeatIntervalSeconds,omitempty" yaml:"repeatIntervalSeconds,omitempty"`
	Severity              string             `json:"severity,omitempty" yaml:"severity,omitempty"`
	SystemServiceRule     *SystemServiceRule `json:"systemServiceRule,omitempty" yaml:"systemServiceRule,omitempty"`
}
