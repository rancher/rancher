package client

const (
	NodeStatusType                 = "nodeStatus"
	NodeStatusFieldAllocatable     = "allocatable"
	NodeStatusFieldCapacity        = "capacity"
	NodeStatusFieldHostname        = "hostname"
	NodeStatusFieldIPAddress       = "ipAddress"
	NodeStatusFieldInfo            = "info"
	NodeStatusFieldNodeConditions  = "nodeConditions"
	NodeStatusFieldVolumesAttached = "volumesAttached"
	NodeStatusFieldVolumesInUse    = "volumesInUse"
)

type NodeStatus struct {
	Allocatable     map[string]string         `json:"allocatable,omitempty"`
	Capacity        map[string]string         `json:"capacity,omitempty"`
	Hostname        string                    `json:"hostname,omitempty"`
	IPAddress       string                    `json:"ipAddress,omitempty"`
	Info            *NodeInfo                 `json:"info,omitempty"`
	NodeConditions  []NodeCondition           `json:"nodeConditions,omitempty"`
	VolumesAttached map[string]AttachedVolume `json:"volumesAttached,omitempty"`
	VolumesInUse    []string                  `json:"volumesInUse,omitempty"`
}
