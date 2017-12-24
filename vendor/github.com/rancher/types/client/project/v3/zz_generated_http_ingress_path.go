package client

const (
	HTTPIngressPathType             = "httpIngressPath"
	HTTPIngressPathFieldPath        = "path"
	HTTPIngressPathFieldServiceId   = "serviceId"
	HTTPIngressPathFieldServicePort = "servicePort"
	HTTPIngressPathFieldWorkloadIDs = "workloadIds"
)

type HTTPIngressPath struct {
	Path        string   `json:"path,omitempty"`
	ServiceId   string   `json:"serviceId,omitempty"`
	ServicePort string   `json:"servicePort,omitempty"`
	WorkloadIDs []string `json:"workloadIds,omitempty"`
}
