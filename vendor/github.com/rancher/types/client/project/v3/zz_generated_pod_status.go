package client

const (
	PodStatusType                       = "podStatus"
	PodStatusFieldConditions            = "conditions"
	PodStatusFieldContainerStatuses     = "containerStatuses"
	PodStatusFieldInitContainerStatuses = "initContainerStatuses"
	PodStatusFieldMessage               = "message"
	PodStatusFieldNodeIp                = "nodeIp"
	PodStatusFieldNominatedNodeName     = "nominatedNodeName"
	PodStatusFieldPhase                 = "phase"
	PodStatusFieldPodIp                 = "podIp"
	PodStatusFieldQOSClass              = "qosClass"
	PodStatusFieldReason                = "reason"
	PodStatusFieldStartTime             = "startTime"
)

type PodStatus struct {
	Conditions            []PodCondition    `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	ContainerStatuses     []ContainerStatus `json:"containerStatuses,omitempty" yaml:"containerStatuses,omitempty"`
	InitContainerStatuses []ContainerStatus `json:"initContainerStatuses,omitempty" yaml:"initContainerStatuses,omitempty"`
	Message               string            `json:"message,omitempty" yaml:"message,omitempty"`
	NodeIp                string            `json:"nodeIp,omitempty" yaml:"nodeIp,omitempty"`
	NominatedNodeName     string            `json:"nominatedNodeName,omitempty" yaml:"nominatedNodeName,omitempty"`
	Phase                 string            `json:"phase,omitempty" yaml:"phase,omitempty"`
	PodIp                 string            `json:"podIp,omitempty" yaml:"podIp,omitempty"`
	QOSClass              string            `json:"qosClass,omitempty" yaml:"qosClass,omitempty"`
	Reason                string            `json:"reason,omitempty" yaml:"reason,omitempty"`
	StartTime             string            `json:"startTime,omitempty" yaml:"startTime,omitempty"`
}
