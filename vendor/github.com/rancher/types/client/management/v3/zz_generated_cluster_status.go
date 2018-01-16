package client

const (
	ClusterStatusType                     = "clusterStatus"
	ClusterStatusFieldAPIEndpoint         = "apiEndpoint"
	ClusterStatusFieldAllocatable         = "allocatable"
	ClusterStatusFieldCACert              = "caCert"
	ClusterStatusFieldCapacity            = "capacity"
	ClusterStatusFieldClusterName         = "clusterName"
	ClusterStatusFieldComponentStatuses   = "componentStatuses"
	ClusterStatusFieldConditions          = "conditions"
	ClusterStatusFieldDriver              = "driver"
	ClusterStatusFieldLimits              = "limits"
	ClusterStatusFieldRequested           = "requested"
	ClusterStatusFieldServiceAccountToken = "serviceAccountToken"
)

type ClusterStatus struct {
	APIEndpoint         string                   `json:"apiEndpoint,omitempty"`
	Allocatable         map[string]string        `json:"allocatable,omitempty"`
	CACert              string                   `json:"caCert,omitempty"`
	Capacity            map[string]string        `json:"capacity,omitempty"`
	ClusterName         string                   `json:"clusterName,omitempty"`
	ComponentStatuses   []ClusterComponentStatus `json:"componentStatuses,omitempty"`
	Conditions          []ClusterCondition       `json:"conditions,omitempty"`
	Driver              string                   `json:"driver,omitempty"`
	Limits              map[string]string        `json:"limits,omitempty"`
	Requested           map[string]string        `json:"requested,omitempty"`
	ServiceAccountToken string                   `json:"serviceAccountToken,omitempty"`
}
