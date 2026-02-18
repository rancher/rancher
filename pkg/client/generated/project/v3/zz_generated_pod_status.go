package client

const (
	PodStatusType                             = "podStatus"
	PodStatusFieldAllocatedResources          = "allocatedResources"
	PodStatusFieldConditions                  = "conditions"
	PodStatusFieldContainerStatuses           = "containerStatuses"
	PodStatusFieldEphemeralContainerStatuses  = "ephemeralContainerStatuses"
	PodStatusFieldExtendedResourceClaimStatus = "extendedResourceClaimStatus"
	PodStatusFieldHostIPs                     = "hostIPs"
	PodStatusFieldInitContainerStatuses       = "initContainerStatuses"
	PodStatusFieldMessage                     = "message"
	PodStatusFieldNodeIp                      = "nodeIp"
	PodStatusFieldNominatedNodeName           = "nominatedNodeName"
	PodStatusFieldObservedGeneration          = "observedGeneration"
	PodStatusFieldPhase                       = "phase"
	PodStatusFieldPodIPs                      = "podIPs"
	PodStatusFieldPodIp                       = "podIp"
	PodStatusFieldQOSClass                    = "qosClass"
	PodStatusFieldReason                      = "reason"
	PodStatusFieldResize                      = "resize"
	PodStatusFieldResourceClaimStatuses       = "resourceClaimStatuses"
	PodStatusFieldResources                   = "resources"
	PodStatusFieldStartTime                   = "startTime"
)

type PodStatus struct {
	AllocatedResources          map[string]string               `json:"allocatedResources,omitempty" yaml:"allocatedResources,omitempty"`
	Conditions                  []PodCondition                  `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	ContainerStatuses           []ContainerStatus               `json:"containerStatuses,omitempty" yaml:"containerStatuses,omitempty"`
	EphemeralContainerStatuses  []ContainerStatus               `json:"ephemeralContainerStatuses,omitempty" yaml:"ephemeralContainerStatuses,omitempty"`
	ExtendedResourceClaimStatus *PodExtendedResourceClaimStatus `json:"extendedResourceClaimStatus,omitempty" yaml:"extendedResourceClaimStatus,omitempty"`
	HostIPs                     []HostIP                        `json:"hostIPs,omitempty" yaml:"hostIPs,omitempty"`
	InitContainerStatuses       []ContainerStatus               `json:"initContainerStatuses,omitempty" yaml:"initContainerStatuses,omitempty"`
	Message                     string                          `json:"message,omitempty" yaml:"message,omitempty"`
	NodeIp                      string                          `json:"nodeIp,omitempty" yaml:"nodeIp,omitempty"`
	NominatedNodeName           string                          `json:"nominatedNodeName,omitempty" yaml:"nominatedNodeName,omitempty"`
	ObservedGeneration          int64                           `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	Phase                       string                          `json:"phase,omitempty" yaml:"phase,omitempty"`
	PodIPs                      []PodIP                         `json:"podIPs,omitempty" yaml:"podIPs,omitempty"`
	PodIp                       string                          `json:"podIp,omitempty" yaml:"podIp,omitempty"`
	QOSClass                    string                          `json:"qosClass,omitempty" yaml:"qosClass,omitempty"`
	Reason                      string                          `json:"reason,omitempty" yaml:"reason,omitempty"`
	Resize                      string                          `json:"resize,omitempty" yaml:"resize,omitempty"`
	ResourceClaimStatuses       []PodResourceClaimStatus        `json:"resourceClaimStatuses,omitempty" yaml:"resourceClaimStatuses,omitempty"`
	Resources                   *ResourceRequirements           `json:"resources,omitempty" yaml:"resources,omitempty"`
	StartTime                   string                          `json:"startTime,omitempty" yaml:"startTime,omitempty"`
}
