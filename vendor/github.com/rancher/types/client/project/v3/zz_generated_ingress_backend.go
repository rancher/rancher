package client

const (
	IngressBackendType             = "ingressBackend"
	IngressBackendFieldServiceId   = "serviceId"
	IngressBackendFieldServicePort = "servicePort"
	IngressBackendFieldWorkloadIDs = "workloadIds"
)

type IngressBackend struct {
	ServiceId   string   `json:"serviceId,omitempty"`
	ServicePort string   `json:"servicePort,omitempty"`
	WorkloadIDs []string `json:"workloadIds,omitempty"`
}
