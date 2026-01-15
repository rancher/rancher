package client

const (
	MachineStatusType                        = "machineStatus"
	MachineStatusFieldAddresses              = "addresses"
	MachineStatusFieldCertificatesExpiryDate = "certificatesExpiryDate"
	MachineStatusFieldConditions             = "conditions"
	MachineStatusFieldDeletion               = "deletion"
	MachineStatusFieldDeprecated             = "deprecated"
	MachineStatusFieldInitialization         = "initialization"
	MachineStatusFieldLastUpdated            = "lastUpdated"
	MachineStatusFieldNodeInfo               = "nodeInfo"
	MachineStatusFieldNodeRef                = "nodeRef"
	MachineStatusFieldObservedGeneration     = "observedGeneration"
	MachineStatusFieldPhase                  = "phase"
)

type MachineStatus struct {
	Addresses              []MachineAddress             `json:"addresses,omitempty" yaml:"addresses,omitempty"`
	CertificatesExpiryDate string                       `json:"certificatesExpiryDate,omitempty" yaml:"certificatesExpiryDate,omitempty"`
	Conditions             []Condition                  `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Deletion               *MachineDeletionStatus       `json:"deletion,omitempty" yaml:"deletion,omitempty"`
	Deprecated             *MachineDeprecatedStatus     `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	Initialization         *MachineInitializationStatus `json:"initialization,omitempty" yaml:"initialization,omitempty"`
	LastUpdated            string                       `json:"lastUpdated,omitempty" yaml:"lastUpdated,omitempty"`
	NodeInfo               *NodeSystemInfo              `json:"nodeInfo,omitempty" yaml:"nodeInfo,omitempty"`
	NodeRef                *MachineNodeReference        `json:"nodeRef,omitempty" yaml:"nodeRef,omitempty"`
	ObservedGeneration     int64                        `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	Phase                  string                       `json:"phase,omitempty" yaml:"phase,omitempty"`
}
