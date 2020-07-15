package client

const (
	AlertmanagerStatusType                     = "alertmanagerStatus"
	AlertmanagerStatusFieldAvailableReplicas   = "availableReplicas"
	AlertmanagerStatusFieldPaused              = "paused"
	AlertmanagerStatusFieldReplicas            = "replicas"
	AlertmanagerStatusFieldUnavailableReplicas = "unavailableReplicas"
	AlertmanagerStatusFieldUpdatedReplicas     = "updatedReplicas"
)

type AlertmanagerStatus struct {
	AvailableReplicas   int64 `json:"availableReplicas,omitempty" yaml:"availableReplicas,omitempty"`
	Paused              bool  `json:"paused,omitempty" yaml:"paused,omitempty"`
	Replicas            int64 `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	UnavailableReplicas int64 `json:"unavailableReplicas,omitempty" yaml:"unavailableReplicas,omitempty"`
	UpdatedReplicas     int64 `json:"updatedReplicas,omitempty" yaml:"updatedReplicas,omitempty"`
}
