package client

const (
	ETCDServiceType                       = "etcdService"
	ETCDServiceFieldBackupConfig          = "backupConfig"
	ETCDServiceFieldCACert                = "caCert"
	ETCDServiceFieldCert                  = "cert"
	ETCDServiceFieldCreation              = "creation"
	ETCDServiceFieldExternalURLs          = "externalUrls"
	ETCDServiceFieldExtraArgs             = "extraArgs"
	ETCDServiceFieldExtraArgsArray        = "extraArgsArray"
	ETCDServiceFieldExtraBinds            = "extraBinds"
	ETCDServiceFieldExtraEnv              = "extraEnv"
	ETCDServiceFieldGID                   = "gid"
	ETCDServiceFieldImage                 = "image"
	ETCDServiceFieldKey                   = "key"
	ETCDServiceFieldPath                  = "path"
	ETCDServiceFieldRetention             = "retention"
	ETCDServiceFieldSnapshot              = "snapshot"
	ETCDServiceFieldUID                   = "uid"
	ETCDServiceFieldWindowsExtraArgs      = "winExtraArgs"
	ETCDServiceFieldWindowsExtraArgsArray = "winExtraArgsArray"
	ETCDServiceFieldWindowsExtraBinds     = "winExtraBinds"
	ETCDServiceFieldWindowsExtraEnv       = "winExtraEnv"
)

type ETCDService struct {
	BackupConfig          *BackupConfig       `json:"backupConfig,omitempty" yaml:"backupConfig,omitempty"`
	CACert                string              `json:"caCert,omitempty" yaml:"caCert,omitempty"`
	Cert                  string              `json:"cert,omitempty" yaml:"cert,omitempty"`
	Creation              string              `json:"creation,omitempty" yaml:"creation,omitempty"`
	ExternalURLs          []string            `json:"externalUrls,omitempty" yaml:"externalUrls,omitempty"`
	ExtraArgs             map[string]string   `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ExtraArgsArray        map[string][]string `json:"extraArgsArray,omitempty" yaml:"extraArgsArray,omitempty"`
	ExtraBinds            []string            `json:"extraBinds,omitempty" yaml:"extraBinds,omitempty"`
	ExtraEnv              []string            `json:"extraEnv,omitempty" yaml:"extraEnv,omitempty"`
	GID                   int64               `json:"gid,omitempty" yaml:"gid,omitempty"`
	Image                 string              `json:"image,omitempty" yaml:"image,omitempty"`
	Key                   string              `json:"key,omitempty" yaml:"key,omitempty"`
	Path                  string              `json:"path,omitempty" yaml:"path,omitempty"`
	Retention             string              `json:"retention,omitempty" yaml:"retention,omitempty"`
	Snapshot              *bool               `json:"snapshot,omitempty" yaml:"snapshot,omitempty"`
	UID                   int64               `json:"uid,omitempty" yaml:"uid,omitempty"`
	WindowsExtraArgs      map[string]string   `json:"winExtraArgs,omitempty" yaml:"winExtraArgs,omitempty"`
	WindowsExtraArgsArray map[string][]string `json:"winExtraArgsArray,omitempty" yaml:"winExtraArgsArray,omitempty"`
	WindowsExtraBinds     []string            `json:"winExtraBinds,omitempty" yaml:"winExtraBinds,omitempty"`
	WindowsExtraEnv       []string            `json:"winExtraEnv,omitempty" yaml:"winExtraEnv,omitempty"`
}
