package client

const (
	MachineStatusType                        = "machineStatus"
	MachineStatusFieldAddresses              = "addresses"
	MachineStatusFieldBootstrapReady         = "bootstrapReady"
	MachineStatusFieldCertificatesExpiryDate = "certificatesExpiryDate"
	MachineStatusFieldConditions             = "conditions"
	MachineStatusFieldFailureMessage         = "failureMessage"
	MachineStatusFieldFailureReason          = "failureReason"
	MachineStatusFieldInfrastructureReady    = "infrastructureReady"
	MachineStatusFieldLastUpdated            = "lastUpdated"
	MachineStatusFieldNodeInfo               = "nodeInfo"
	MachineStatusFieldNodeRef                = "nodeRef"
	MachineStatusFieldObservedGeneration     = "observedGeneration"
	MachineStatusFieldPhase                  = "phase"
)

type MachineStatus struct {
	Addresses              []MachineAddress `json:"addresses,omitempty" yaml:"addresses,omitempty"`
	BootstrapReady         bool             `json:"bootstrapReady,omitempty" yaml:"bootstrapReady,omitempty"`
	CertificatesExpiryDate string           `json:"certificatesExpiryDate,omitempty" yaml:"certificatesExpiryDate,omitempty"`
	Conditions             []Condition      `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	FailureMessage         string           `json:"failureMessage,omitempty" yaml:"failureMessage,omitempty"`
	FailureReason          string           `json:"failureReason,omitempty" yaml:"failureReason,omitempty"`
	InfrastructureReady    bool             `json:"infrastructureReady,omitempty" yaml:"infrastructureReady,omitempty"`
	LastUpdated            string           `json:"lastUpdated,omitempty" yaml:"lastUpdated,omitempty"`
	NodeInfo               *NodeSystemInfo  `json:"nodeInfo,omitempty" yaml:"nodeInfo,omitempty"`
	NodeRef                *ObjectReference `json:"nodeRef,omitempty" yaml:"nodeRef,omitempty"`
	ObservedGeneration     int64            `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	Phase                  string           `json:"phase,omitempty" yaml:"phase,omitempty"`
}
