package client

const (
	MachineStatusType                 = "machineStatus"
	MachineStatusFieldAllocatable     = "allocatable"
	MachineStatusFieldCapacity        = "capacity"
	MachineStatusFieldConditions      = "conditions"
	MachineStatusFieldHostname        = "hostname"
	MachineStatusFieldIPAddress       = "ipAddress"
	MachineStatusFieldInfo            = "info"
	MachineStatusFieldLimits          = "limits"
	MachineStatusFieldNodeAnnotations = "nodeAnnotations"
	MachineStatusFieldNodeLabels      = "nodeLabels"
	MachineStatusFieldNodeName        = "nodeName"
	MachineStatusFieldNodeTaints      = "nodeTaints"
	MachineStatusFieldRequested       = "requested"
	MachineStatusFieldSSHUser         = "sshUser"
	MachineStatusFieldVolumesAttached = "volumesAttached"
	MachineStatusFieldVolumesInUse    = "volumesInUse"
)

type MachineStatus struct {
	Allocatable     map[string]string         `json:"allocatable,omitempty"`
	Capacity        map[string]string         `json:"capacity,omitempty"`
	Conditions      []MachineCondition        `json:"conditions,omitempty"`
	Hostname        string                    `json:"hostname,omitempty"`
	IPAddress       string                    `json:"ipAddress,omitempty"`
	Info            *NodeInfo                 `json:"info,omitempty"`
	Limits          map[string]string         `json:"limits,omitempty"`
	NodeAnnotations map[string]string         `json:"nodeAnnotations,omitempty"`
	NodeLabels      map[string]string         `json:"nodeLabels,omitempty"`
	NodeName        string                    `json:"nodeName,omitempty"`
	NodeTaints      []Taint                   `json:"nodeTaints,omitempty"`
	Requested       map[string]string         `json:"requested,omitempty"`
	SSHUser         string                    `json:"sshUser,omitempty"`
	VolumesAttached map[string]AttachedVolume `json:"volumesAttached,omitempty"`
	VolumesInUse    []string                  `json:"volumesInUse,omitempty"`
}
