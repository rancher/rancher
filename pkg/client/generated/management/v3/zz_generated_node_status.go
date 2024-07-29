package client

const (
	NodeStatusType                    = "nodeStatus"
	NodeStatusFieldAllocatable        = "allocatable"
	NodeStatusFieldAppliedNodeVersion = "appliedNodeVersion"
	NodeStatusFieldCapacity           = "capacity"
	NodeStatusFieldConditions         = "conditions"
	NodeStatusFieldDockerInfo         = "dockerInfo"
	NodeStatusFieldExternalIPAddress  = "externalIpAddress"
	NodeStatusFieldHostname           = "hostname"
	NodeStatusFieldIPAddress          = "ipAddress"
	NodeStatusFieldInfo               = "info"
	NodeStatusFieldLimits             = "limits"
	NodeStatusFieldNodeAnnotations    = "nodeAnnotations"
	NodeStatusFieldNodeConfig         = "rkeNode"
	NodeStatusFieldNodeLabels         = "nodeLabels"
	NodeStatusFieldNodeName           = "nodeName"
	NodeStatusFieldNodePlan           = "nodePlan"
	NodeStatusFieldNodeTaints         = "nodeTaints"
	NodeStatusFieldRequested          = "requested"
	NodeStatusFieldRuntimeHandlers    = "runtimeHandlers"
	NodeStatusFieldVolumesAttached    = "volumesAttached"
	NodeStatusFieldVolumesInUse       = "volumesInUse"
)

type NodeStatus struct {
	Allocatable        map[string]string         `json:"allocatable,omitempty" yaml:"allocatable,omitempty"`
	AppliedNodeVersion int64                     `json:"appliedNodeVersion,omitempty" yaml:"appliedNodeVersion,omitempty"`
	Capacity           map[string]string         `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	Conditions         []NodeCondition           `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	DockerInfo         *DockerInfo               `json:"dockerInfo,omitempty" yaml:"dockerInfo,omitempty"`
	ExternalIPAddress  string                    `json:"externalIpAddress,omitempty" yaml:"externalIpAddress,omitempty"`
	Hostname           string                    `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	IPAddress          string                    `json:"ipAddress,omitempty" yaml:"ipAddress,omitempty"`
	Info               *NodeInfo                 `json:"info,omitempty" yaml:"info,omitempty"`
	Limits             map[string]string         `json:"limits,omitempty" yaml:"limits,omitempty"`
	NodeAnnotations    map[string]string         `json:"nodeAnnotations,omitempty" yaml:"nodeAnnotations,omitempty"`
	NodeConfig         *RKEConfigNode            `json:"rkeNode,omitempty" yaml:"rkeNode,omitempty"`
	NodeLabels         map[string]string         `json:"nodeLabels,omitempty" yaml:"nodeLabels,omitempty"`
	NodeName           string                    `json:"nodeName,omitempty" yaml:"nodeName,omitempty"`
	NodePlan           *NodePlan                 `json:"nodePlan,omitempty" yaml:"nodePlan,omitempty"`
	NodeTaints         []Taint                   `json:"nodeTaints,omitempty" yaml:"nodeTaints,omitempty"`
	Requested          map[string]string         `json:"requested,omitempty" yaml:"requested,omitempty"`
	RuntimeHandlers    []NodeRuntimeHandler      `json:"runtimeHandlers,omitempty" yaml:"runtimeHandlers,omitempty"`
	VolumesAttached    map[string]AttachedVolume `json:"volumesAttached,omitempty" yaml:"volumesAttached,omitempty"`
	VolumesInUse       []string                  `json:"volumesInUse,omitempty" yaml:"volumesInUse,omitempty"`
}
