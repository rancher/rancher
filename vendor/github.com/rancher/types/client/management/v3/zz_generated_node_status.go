package client

const (
	NodeStatusType                 = "nodeStatus"
	NodeStatusFieldAllocatable     = "allocatable"
	NodeStatusFieldCapacity        = "capacity"
	NodeStatusFieldConditions      = "conditions"
	NodeStatusFieldHostname        = "hostname"
	NodeStatusFieldIPAddress       = "ipAddress"
	NodeStatusFieldInfo            = "info"
	NodeStatusFieldPhase           = "phase"
	NodeStatusFieldVolumesAttached = "volumesAttached"
	NodeStatusFieldVolumesInUse    = "volumesInUse"
)

type NodeStatus struct {
	Allocatable     map[string]string         `json:"allocatable,omitempty"`
	Capacity        map[string]string         `json:"capacity,omitempty"`
	Conditions      []NodeCondition           `json:"conditions,omitempty"`
	Hostname        string                    `json:"hostname,omitempty"`
	IPAddress       string                    `json:"ipAddress,omitempty"`
	Info            *NodeInfo                 `json:"info,omitempty"`
	Phase           string                    `json:"phase,omitempty"`
	VolumesAttached map[string]AttachedVolume `json:"volumesAttached,omitempty"`
	VolumesInUse    []string                  `json:"volumesInUse,omitempty"`
}
