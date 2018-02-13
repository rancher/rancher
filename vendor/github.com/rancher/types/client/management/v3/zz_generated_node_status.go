package client

const (
	NodeStatusType                 = "nodeStatus"
	NodeStatusFieldAllocatable     = "allocatable"
	NodeStatusFieldCapacity        = "capacity"
	NodeStatusFieldConditions      = "conditions"
	NodeStatusFieldHostname        = "hostname"
	NodeStatusFieldIPAddress       = "ipAddress"
	NodeStatusFieldInfo            = "info"
	NodeStatusFieldLimits          = "limits"
	NodeStatusFieldNodeAnnotations = "nodeAnnotations"
	NodeStatusFieldNodeConfig      = "rkeNode"
	NodeStatusFieldNodeLabels      = "nodeLabels"
	NodeStatusFieldNodeName        = "nodeName"
	NodeStatusFieldNodeTaints      = "nodeTaints"
	NodeStatusFieldRequested       = "requested"
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
	Limits          map[string]string         `json:"limits,omitempty"`
	NodeAnnotations map[string]string         `json:"nodeAnnotations,omitempty"`
	NodeConfig      *RKEConfigNode            `json:"rkeNode,omitempty"`
	NodeLabels      map[string]string         `json:"nodeLabels,omitempty"`
	NodeName        string                    `json:"nodeName,omitempty"`
	NodeTaints      []Taint                   `json:"nodeTaints,omitempty"`
	Requested       map[string]string         `json:"requested,omitempty"`
	VolumesAttached map[string]AttachedVolume `json:"volumesAttached,omitempty"`
	VolumesInUse    []string                  `json:"volumesInUse,omitempty"`
}
