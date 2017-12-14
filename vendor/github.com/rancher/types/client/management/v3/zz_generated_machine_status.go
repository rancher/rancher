package client

const (
	MachineStatusType                 = "machineStatus"
	MachineStatusFieldAddress         = "address"
	MachineStatusFieldAllocatable     = "allocatable"
	MachineStatusFieldCapacity        = "capacity"
	MachineStatusFieldConditions      = "conditions"
	MachineStatusFieldExtractedConfig = "extractedConfig"
	MachineStatusFieldHostname        = "hostname"
	MachineStatusFieldIPAddress       = "ipAddress"
	MachineStatusFieldInfo            = "info"
	MachineStatusFieldLimits          = "limits"
	MachineStatusFieldNodeName        = "nodeName"
	MachineStatusFieldPhase           = "phase"
	MachineStatusFieldProvisioned     = "provisioned"
	MachineStatusFieldRequested       = "requested"
	MachineStatusFieldSSHPrivateKey   = "sshPrivateKey"
	MachineStatusFieldSSHUser         = "sshUser"
	MachineStatusFieldVolumesAttached = "volumesAttached"
	MachineStatusFieldVolumesInUse    = "volumesInUse"
)

type MachineStatus struct {
	Address         string                    `json:"address,omitempty"`
	Allocatable     map[string]string         `json:"allocatable,omitempty"`
	Capacity        map[string]string         `json:"capacity,omitempty"`
	Conditions      []NodeCondition           `json:"conditions,omitempty"`
	ExtractedConfig string                    `json:"extractedConfig,omitempty"`
	Hostname        string                    `json:"hostname,omitempty"`
	IPAddress       string                    `json:"ipAddress,omitempty"`
	Info            *NodeInfo                 `json:"info,omitempty"`
	Limits          map[string]string         `json:"limits,omitempty"`
	NodeName        string                    `json:"nodeName,omitempty"`
	Phase           string                    `json:"phase,omitempty"`
	Provisioned     *bool                     `json:"provisioned,omitempty"`
	Requested       map[string]string         `json:"requested,omitempty"`
	SSHPrivateKey   string                    `json:"sshPrivateKey,omitempty"`
	SSHUser         string                    `json:"sshUser,omitempty"`
	VolumesAttached map[string]AttachedVolume `json:"volumesAttached,omitempty"`
	VolumesInUse    []string                  `json:"volumesInUse,omitempty"`
}
