package client

const (
	ClusterStatusType                   = "clusterStatus"
	ClusterStatusFieldAPIEndpoint       = "apiEndpoint"
	ClusterStatusFieldAllocatable       = "allocatable"
	ClusterStatusFieldCACert            = "caCert"
	ClusterStatusFieldCapacity          = "capacity"
	ClusterStatusFieldComponentStatuses = "componentStatuses"
	ClusterStatusFieldConditions        = "conditions"
	ClusterStatusFieldDriver            = "driver"
	ClusterStatusFieldFailedSpec        = "failedSpec"
	ClusterStatusFieldLimits            = "limits"
	ClusterStatusFieldRequested         = "requested"
)

type ClusterStatus struct {
	APIEndpoint       string                   `json:"apiEndpoint,omitempty"`
	Allocatable       map[string]string        `json:"allocatable,omitempty"`
	CACert            string                   `json:"caCert,omitempty"`
	Capacity          map[string]string        `json:"capacity,omitempty"`
	ComponentStatuses []ClusterComponentStatus `json:"componentStatuses,omitempty"`
	Conditions        []ClusterCondition       `json:"conditions,omitempty"`
	Driver            string                   `json:"driver,omitempty"`
	FailedSpec        *ClusterSpec             `json:"failedSpec,omitempty"`
	Limits            map[string]string        `json:"limits,omitempty"`
	Requested         map[string]string        `json:"requested,omitempty"`
}
