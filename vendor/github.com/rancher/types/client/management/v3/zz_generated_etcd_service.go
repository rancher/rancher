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
	CACert       string            `json:"caCert,omitempty"`
	Cert         string            `json:"cert,omitempty"`
	ExternalURLs []string          `json:"externalUrls,omitempty"`
	ExtraArgs    map[string]string `json:"extraArgs,omitempty"`
	Image        string            `json:"image,omitempty"`
	Key          string            `json:"key,omitempty"`
	Path         string            `json:"path,omitempty"`
}
