package store

type KubeConfig struct {
	APIVersion     string            `yaml:"apiVersion,omitempty"`
	Clusters       []ConfigCluster   `yaml:"clusters,omitempty"`
	Contexts       []ConfigContext   `yaml:"contexts,omitempty"`
	Users          []ConfigUser      `yaml:"users,omitempty"`
	CurrentContext string            `yaml:"current-context,omitempty"`
	Kind           string            `yaml:"kind,omitempty"`
	Preferences    map[string]string `yaml:"preferences,omitempty"`
}

type ConfigCluster struct {
	Cluster DataCluster `yaml:"cluster,omitempty"`
	Name    string      `yaml:"name,omitempty"`
}

type DataCluster struct {
	CertificateAuthority     string `yaml:"certificate-authority,omitempty"`
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	Server                   string `yaml:"server,omitempty"`
}

type ConfigContext struct {
	Context ContextData `yaml:"context,omitempty"`
	Name    string      `yaml:"name,omitempty"`
}

type ContextData struct {
	Cluster string `yaml:"cluster,omitempty"`
	User    string `yaml:"user,omitempty"`
}

type ConfigUser struct {
	Name string   `yaml:"name,omitempty"`
	User UserData `yaml:"user,omitempty"`
}

type UserData struct {
	Token                 string `yaml:"token,omitempty"`
	Username              string `yaml:"username,omitempty"`
	Password              string `yaml:"password,omitempty"`
	ClientCertificateData string `yaml:"client-certificate-data,omitempty"`
	ClientKeyData         string `yaml:"client-key-data,omitempty"`
}
