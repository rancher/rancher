package client

const (
	IngressBackendType             = "ingressBackend"
	IngressBackendFieldServiceId   = "serviceId"
	IngressBackendFieldTargetPort  = "targetPort"
	IngressBackendFieldWorkloadIDs = "workloadIds"
)

type IngressBackend struct {
	ServiceId   string   `json:"serviceId,omitempty"`
	TargetPort  string   `json:"targetPort,omitempty"`
	WorkloadIDs []string `json:"workloadIds,omitempty"`
}
