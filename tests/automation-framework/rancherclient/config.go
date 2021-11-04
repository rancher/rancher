package rancherclient

type Config struct {
	CattleTestURL     string `json:"cattleTestURL"`
	AdminToken        string `json:"adminToken"`
	UserToken         string `json:"userToken,omitempty"`
	CNI               string `json:"cni" default:"calico"`
	KubernetesVersion string `json:"kubernetesVersion"`
	NodeRoles         string `json:"nodeRoles,omitempty"`
	DefaultNamespace  string `default:"fleet-default"`
	Insecure          *bool  `json:"insecure" default:"true"`
	CAFile            string `json:"caFile" default:""`
	CACerts           string `json:"caCerts" default:""`
}

func (r *Config) SetCattleTestURL(url string) {
	r.CattleTestURL = url
}

func (r *Config) GetCattleTestURL() string {
	return r.CattleTestURL
}

func (r *Config) SetAdminToken(token string) {
	r.AdminToken = token
}

func (r *Config) GetAdminToken() string {
	return r.AdminToken
}

func (r *Config) SetUserToken(token string) {
	r.UserToken = token
}

func (r *Config) GetUserToken() string {
	return r.UserToken
}

func (r *Config) SetCNI(cleanup string) {
	r.CNI = cleanup
}

func (r *Config) GetCNI() string {
	return r.CNI
}

func (r *Config) SetKubernetesVersion(kubernetesVersion string) {
	r.KubernetesVersion = kubernetesVersion
}

func (r *Config) GetKubernetesVersion() string {
	return r.KubernetesVersion
}

func (r *Config) SetNodeRoles(nodeRoles string) {
	r.NodeRoles = nodeRoles
}

func (r *Config) GetNodeRoles() string {
	return r.NodeRoles
}

func (r *Config) SetDefaultNamespace(namespace string) {
	r.DefaultNamespace = namespace
}

func (r *Config) GetDefaultNamespace() string {
	return r.DefaultNamespace
}

func (r *Config) SetInsecure(insecure bool) {
	r.Insecure = &insecure
}

func (r *Config) GetInsecure() bool {
	return *r.Insecure
}

func (r *Config) SetCAFile(caFile string) {
	r.CAFile = caFile
}

func (r *Config) GetCAFile() string {
	return r.CAFile
}

func (r *Config) SetCACerts(caCerts string) {
	r.CACerts = caCerts
}

func (r *Config) GetCACerts() string {
	return r.CACerts
}
