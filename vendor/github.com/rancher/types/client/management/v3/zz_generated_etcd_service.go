package client

const (
	ETCDServiceType              = "etcdService"
	ETCDServiceFieldCACert       = "caCert"
	ETCDServiceFieldCert         = "cert"
	ETCDServiceFieldExternalURLs = "externalUrls"
	ETCDServiceFieldExtraArgs    = "extraArgs"
	ETCDServiceFieldImage        = "image"
	ETCDServiceFieldKey          = "key"
	ETCDServiceFieldPath         = "path"
)

type ETCDService struct {
	CACert       string            `json:"caCert,omitempty" yaml:"caCert,omitempty"`
	Cert         string            `json:"cert,omitempty" yaml:"cert,omitempty"`
	ExternalURLs []string          `json:"externalUrls,omitempty" yaml:"externalUrls,omitempty"`
	ExtraArgs    map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	Image        string            `json:"image,omitempty" yaml:"image,omitempty"`
	Key          string            `json:"key,omitempty" yaml:"key,omitempty"`
	Path         string            `json:"path,omitempty" yaml:"path,omitempty"`
}
