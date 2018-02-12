package client

const (
	ClusterAlertSpecType                       = "clusterAlertSpec"
	ClusterAlertSpecFieldClusterId             = "clusterId"
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
	ClusterId             string               `json:"clusterId,omitempty"`
	Description           string               `json:"description,omitempty"`
	DisplayName           string               `json:"displayName,omitempty"`
	InitialWaitSeconds    *int64               `json:"initialWaitSeconds,omitempty"`
	Recipients            []Recipient          `json:"recipients,omitempty"`
	RepeatIntervalSeconds *int64               `json:"repeatIntervalSeconds,omitempty"`
	Severity              string               `json:"severity,omitempty"`
	TargetEvent           *TargetEvent         `json:"targetEvent,omitempty"`
	TargetNode            *TargetNode          `json:"targetNode,omitempty"`
	TargetSystemService   *TargetSystemService `json:"targetSystemService,omitempty"`
}
