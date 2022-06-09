package client

const (
	ClusterAlertSpecType                       = "clusterAlertSpec"
	ClusterAlertSpecFieldClusterID             = "clusterId"
	ClusterAlertSpecFieldDescription           = "description"
	ClusterAlertSpecFieldDisplayName           = "displayName"
	ClusterAlertSpecFieldInitialWaitSeconds    = "initialWaitSeconds"
	ClusterAlertSpecFieldRecipients            = "recipients"
	ClusterAlertSpecFieldRepeatIntervalSeconds = "repeatIntervalSeconds"
	ClusterAlertSpecFieldSeverity              = "severity"
	ClusterAlertSpecFieldTargetEvent           = "targetEvent"
	ClusterAlertSpecFieldTargetNode            = "targetNode"
	ClusterAlertSpecFieldTargetSystemService   = "targetSystemService"
)

type ClusterAlertSpec struct {
	ClusterID             string               `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Description           string               `json:"description,omitempty" yaml:"description,omitempty"`
	DisplayName           string               `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	InitialWaitSeconds    int64                `json:"initialWaitSeconds,omitempty" yaml:"initialWaitSeconds,omitempty"`
	Recipients            []Recipient          `json:"recipients,omitempty" yaml:"recipients,omitempty"`
	RepeatIntervalSeconds int64                `json:"repeatIntervalSeconds,omitempty" yaml:"repeatIntervalSeconds,omitempty"`
	Severity              string               `json:"severity,omitempty" yaml:"severity,omitempty"`
	TargetEvent           *TargetEvent         `json:"targetEvent,omitempty" yaml:"targetEvent,omitempty"`
	TargetNode            *TargetNode          `json:"targetNode,omitempty" yaml:"targetNode,omitempty"`
	TargetSystemService   *TargetSystemService `json:"targetSystemService,omitempty" yaml:"targetSystemService,omitempty"`
}
