package client

const (
	K8sServerConfigType                      = "k8sServerConfig"
	K8sServerConfigFieldAdmissionControllers = "admissionControllers"
	K8sServerConfigFieldServiceNetCIDR       = "serviceNetCidr"
)

type K8sServerConfig struct {
	AdmissionControllers []string `json:"admissionControllers,omitempty"`
	ServiceNetCIDR       string   `json:"serviceNetCidr,omitempty"`
}
