package client

const (
	InternalNodeStatusType                 = "internalNodeStatus"
	InternalNodeStatusFieldAllocatable     = "allocatable"
	InternalNodeStatusFieldCapacity        = "capacity"
	InternalNodeStatusFieldHostname        = "hostname"
	InternalNodeStatusFieldIPAddress       = "ipAddress"
	InternalNodeStatusFieldInfo            = "info"
	InternalNodeStatusFieldNodeConditions  = "nodeConditions"
	InternalNodeStatusFieldVolumesAttached = "volumesAttached"
	InternalNodeStatusFieldVolumesInUse    = "volumesInUse"
)

type InternalNodeStatus struct {
	Allocatable     map[string]string         `json:"allocatable,omitempty"`
	Capacity        map[string]string         `json:"capacity,omitempty"`
	Hostname        string                    `json:"hostname,omitempty"`
	IPAddress       string                    `json:"ipAddress,omitempty"`
	Info            *NodeInfo                 `json:"info,omitempty"`
	NodeConditions  []NodeCondition           `json:"nodeConditions,omitempty"`
	VolumesAttached map[string]AttachedVolume `json:"volumesAttached,omitempty"`
	VolumesInUse    []string                  `json:"volumesInUse,omitempty"`
}
