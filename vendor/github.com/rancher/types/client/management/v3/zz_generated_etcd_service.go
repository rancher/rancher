package client

const (
	ETCDServiceType              = "etcdService"
	ETCDServiceFieldBackup       = "backup"
	ETCDServiceFieldCACert       = "caCert"
	ETCDServiceFieldCert         = "cert"
	ETCDServiceFieldCreation     = "creation"
	ETCDServiceFieldExternalURLs = "externalUrls"
	ETCDServiceFieldExtraArgs    = "extraArgs"
	ETCDServiceFieldExtraBinds   = "extraBinds"
	ETCDServiceFieldImage        = "image"
	ETCDServiceFieldKey          = "key"
	ETCDServiceFieldPath         = "path"
	ETCDServiceFieldRetention    = "retention"
)

type ETCDService struct {
	Backup       bool              `json:"backup,omitempty" yaml:"backup,omitempty"`
	CACert       string            `json:"caCert,omitempty" yaml:"caCert,omitempty"`
	Cert         string            `json:"cert,omitempty" yaml:"cert,omitempty"`
	Creation     string            `json:"creation,omitempty" yaml:"creation,omitempty"`
	ExternalURLs []string          `json:"externalUrls,omitempty" yaml:"externalUrls,omitempty"`
	ExtraArgs    map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ExtraBinds   []string          `json:"extraBinds,omitempty" yaml:"extraBinds,omitempty"`
	Image        string            `json:"image,omitempty" yaml:"image,omitempty"`
	Key          string            `json:"key,omitempty" yaml:"key,omitempty"`
	Path         string            `json:"path,omitempty" yaml:"path,omitempty"`
	Retention    string            `json:"retention,omitempty" yaml:"retention,omitempty"`
}
