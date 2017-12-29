package client

const (
	MachineStatusType                 = "machineStatus"
	MachineStatusFieldAllocatable     = "allocatable"
	MachineStatusFieldCapacity        = "capacity"
	MachineStatusFieldClusterId       = "clusterId"
	MachineStatusFieldHostname        = "hostname"
	MachineStatusFieldIPAddress       = "ipAddress"
	MachineStatusFieldInfo            = "info"
	MachineStatusFieldLimits          = "limits"
	MachineStatusFieldNodeName        = "nodeName"
	MachineStatusFieldRequested       = "requested"
	MachineStatusFieldSSHUser         = "sshUser"
	MachineStatusFieldVolumesAttached = "volumesAttached"
	MachineStatusFieldVolumesInUse    = "volumesInUse"
)

type MachineStatus struct {
	Allocatable     map[string]string         `json:"allocatable,omitempty"`
	Capacity        map[string]string         `json:"capacity,omitempty"`
	ClusterId       string                    `json:"clusterId,omitempty"`
	Hostname        string                    `json:"hostname,omitempty"`
	IPAddress       string                    `json:"ipAddress,omitempty"`
	Info            *NodeInfo                 `json:"info,omitempty"`
	Limits          map[string]string         `json:"limits,omitempty"`
	NodeName        string                    `json:"nodeName,omitempty"`
	Requested       map[string]string         `json:"requested,omitempty"`
	SSHUser         string                    `json:"sshUser,omitempty"`
	VolumesAttached map[string]AttachedVolume `json:"volumesAttached,omitempty"`
	VolumesInUse    []string                  `json:"volumesInUse,omitempty"`
}
