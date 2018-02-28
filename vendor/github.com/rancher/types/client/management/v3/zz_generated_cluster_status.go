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
	APIEndpoint       string                   `json:"apiEndpoint,omitempty" yaml:"apiEndpoint,omitempty"`
	Allocatable       map[string]string        `json:"allocatable,omitempty" yaml:"allocatable,omitempty"`
	CACert            string                   `json:"caCert,omitempty" yaml:"caCert,omitempty"`
	Capacity          map[string]string        `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	ComponentStatuses []ClusterComponentStatus `json:"componentStatuses,omitempty" yaml:"componentStatuses,omitempty"`
	Conditions        []ClusterCondition       `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Driver            string                   `json:"driver,omitempty" yaml:"driver,omitempty"`
	FailedSpec        *ClusterSpec             `json:"failedSpec,omitempty" yaml:"failedSpec,omitempty"`
	Limits            map[string]string        `json:"limits,omitempty" yaml:"limits,omitempty"`
	Requested         map[string]string        `json:"requested,omitempty" yaml:"requested,omitempty"`
}
