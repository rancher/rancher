package client

const (
	InternalNodeStatusType                     = "internalNodeStatus"
	InternalNodeStatusFieldAllocatable         = "allocatable"
	InternalNodeStatusFieldCapacity            = "capacity"
	InternalNodeStatusFieldConfig              = "config"
	InternalNodeStatusFieldDeclaredFeatures    = "declaredFeatures"
	InternalNodeStatusFieldExternalIPAddress   = "externalIpAddress"
	InternalNodeStatusFieldExternalIPv4Address = "externalIpv4Address"
	InternalNodeStatusFieldExternalIPv6Address = "externalIpv6Address"
	InternalNodeStatusFieldFeatures            = "features"
	InternalNodeStatusFieldHostname            = "hostname"
	InternalNodeStatusFieldIPAddress           = "ipAddress"
	InternalNodeStatusFieldIPv4Address         = "ipv4Address"
	InternalNodeStatusFieldIPv6Address         = "ipv6Address"
	InternalNodeStatusFieldInfo                = "info"
	InternalNodeStatusFieldNodeConditions      = "nodeConditions"
	InternalNodeStatusFieldRuntimeHandlers     = "runtimeHandlers"
	InternalNodeStatusFieldVolumesAttached     = "volumesAttached"
	InternalNodeStatusFieldVolumesInUse        = "volumesInUse"
)

type InternalNodeStatus struct {
	Allocatable         map[string]string         `json:"allocatable,omitempty" yaml:"allocatable,omitempty"`
	Capacity            map[string]string         `json:"capacity,omitempty" yaml:"capacity,omitempty"`
	Config              *NodeConfigStatus         `json:"config,omitempty" yaml:"config,omitempty"`
	DeclaredFeatures    []string                  `json:"declaredFeatures,omitempty" yaml:"declaredFeatures,omitempty"`
	ExternalIPAddress   string                    `json:"externalIpAddress,omitempty" yaml:"externalIpAddress,omitempty"`
	ExternalIPv4Address string                    `json:"externalIpv4Address,omitempty" yaml:"externalIpv4Address,omitempty"`
	ExternalIPv6Address string                    `json:"externalIpv6Address,omitempty" yaml:"externalIpv6Address,omitempty"`
	Features            *NodeFeatures             `json:"features,omitempty" yaml:"features,omitempty"`
	Hostname            string                    `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	IPAddress           string                    `json:"ipAddress,omitempty" yaml:"ipAddress,omitempty"`
	IPv4Address         string                    `json:"ipv4Address,omitempty" yaml:"ipv4Address,omitempty"`
	IPv6Address         string                    `json:"ipv6Address,omitempty" yaml:"ipv6Address,omitempty"`
	Info                *NodeInfo                 `json:"info,omitempty" yaml:"info,omitempty"`
	NodeConditions      []NodeCondition           `json:"nodeConditions,omitempty" yaml:"nodeConditions,omitempty"`
	RuntimeHandlers     []NodeRuntimeHandler      `json:"runtimeHandlers,omitempty" yaml:"runtimeHandlers,omitempty"`
	VolumesAttached     map[string]AttachedVolume `json:"volumesAttached,omitempty" yaml:"volumesAttached,omitempty"`
	VolumesInUse        []string                  `json:"volumesInUse,omitempty" yaml:"volumesInUse,omitempty"`
}
