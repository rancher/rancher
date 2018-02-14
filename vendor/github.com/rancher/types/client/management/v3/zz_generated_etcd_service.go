package client

const (
	ETCDServiceType             = "etcdService"
	ETCDServiceFieldCACert      = "caCert"
	ETCDServiceFieldCert        = "cert"
	ETCDServiceFieldEtcdServers = "etcdServers"
	ETCDServiceFieldExtraArgs   = "extraArgs"
	ETCDServiceFieldImage       = "image"
	ETCDServiceFieldKey         = "key"
	ETCDServiceFieldPath        = "path"
)

type ETCDService struct {
	CACert      string            `json:"caCert,omitempty"`
	Cert        string            `json:"cert,omitempty"`
	EtcdServers string            `json:"etcdServers,omitempty"`
	ExtraArgs   map[string]string `json:"extraArgs,omitempty"`
	Image       string            `json:"image,omitempty"`
	Key         string            `json:"key,omitempty"`
	Path        string            `json:"path,omitempty"`
}
