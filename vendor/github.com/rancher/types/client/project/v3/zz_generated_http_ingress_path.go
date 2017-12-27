package client

const (
	HTTPIngressPathType             = "httpIngressPath"
	HTTPIngressPathFieldServiceId   = "serviceId"
	HTTPIngressPathFieldTargetPort  = "targetPort"
	HTTPIngressPathFieldWorkloadIDs = "workloadIds"
)

type HTTPIngressPath struct {
	ServiceId   string   `json:"serviceId,omitempty"`
	TargetPort  string   `json:"targetPort,omitempty"`
	WorkloadIDs []string `json:"workloadIds,omitempty"`
}
