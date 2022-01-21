package v1

type RotateCertificates struct {
	Generation int64 `json:"generation,omitempty"`

	CACertificates bool     `json:"caCertificates,omitempty"`
	Services       []string `json:"services,omitempty" norman:"type=enum,options=admin|api-server|controller-manager|scheduler|rke2-controller|rke2-server|cloud-controller|etcd|auth-proxy|kubelet|kube-proxy"`
}
