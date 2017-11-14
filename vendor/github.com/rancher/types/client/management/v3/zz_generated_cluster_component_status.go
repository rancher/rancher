package client

const (
	ClusterComponentStatusType            = "clusterComponentStatus"
	ClusterComponentStatusFieldConditions = "conditions"
	ClusterComponentStatusFieldName       = "name"
)

type ClusterComponentStatus struct {
	Conditions []ComponentCondition `json:"conditions,omitempty"`
	Name       string               `json:"name,omitempty"`
}
