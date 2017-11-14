package client

const (
	PodStatusType                       = "podStatus"
	PodStatusFieldConditions            = "conditions"
	PodStatusFieldContainerStatuses     = "containerStatuses"
	PodStatusFieldInitContainerStatuses = "initContainerStatuses"
	PodStatusFieldMessage               = "message"
	PodStatusFieldNodeIp                = "nodeIp"
	PodStatusFieldPhase                 = "phase"
	PodStatusFieldPodIp                 = "podIp"
	PodStatusFieldQOSClass              = "qosClass"
	PodStatusFieldReason                = "reason"
	PodStatusFieldStartTime             = "startTime"
)

type PodStatus struct {
	Conditions            []PodCondition    `json:"conditions,omitempty"`
	ContainerStatuses     []ContainerStatus `json:"containerStatuses,omitempty"`
	InitContainerStatuses []ContainerStatus `json:"initContainerStatuses,omitempty"`
	Message               string            `json:"message,omitempty"`
	NodeIp                string            `json:"nodeIp,omitempty"`
	Phase                 string            `json:"phase,omitempty"`
	PodIp                 string            `json:"podIp,omitempty"`
	QOSClass              string            `json:"qosClass,omitempty"`
	Reason                string            `json:"reason,omitempty"`
	StartTime             string            `json:"startTime,omitempty"`
}
