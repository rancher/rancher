package config

func (r *RancherServerConfig) SetCattleTestURL(url string) {
	r.CattleTestURL = url
}

func (r *RancherServerConfig) GetCattleTestURL() string {
	return r.CattleTestURL
}

func (r *RancherServerConfig) SetAdminToken(token string) {
	r.AdminToken = token
}

func (r *RancherServerConfig) GetAdminToken() string {
	return r.AdminToken
}

func (r *RancherServerConfig) SetUserToken(token string) {
	r.UserToken = token
}

func (r *RancherServerConfig) GetUserToken() string {
	return r.UserToken
}

func (r *RancherServerConfig) SetCNI(cleanup string) {
	r.CNI = cleanup
}

func (r *RancherServerConfig) GetCNI() string {
	return r.CNI
}

func (r *RancherServerConfig) SetKubernetesVersion(kubernetesVersion string) {
	r.KubernetesVersion = kubernetesVersion
}

func (r *RancherServerConfig) GetKubernetesVersion() string {
	return r.KubernetesVersion
}

func (r *RancherServerConfig) SetNodeRoles(nodeRoles string) {
	r.NodeRoles = nodeRoles
}

func (r *RancherServerConfig) GetNodeRoles() string {
	return r.NodeRoles
}

func (r *RancherServerConfig) SetDefaultNamespace(namespace string) {
	r.DefaultNamespace = namespace
}

func (r *RancherServerConfig) GetDefaultNamespace() string {
	return r.DefaultNamespace
}

func (r *RancherServerConfig) SetInsecure(insecure bool) {
	r.Insecure = &insecure
}

func (r *RancherServerConfig) GetInsecure() bool {
	return *r.Insecure
}

func (r *RancherServerConfig) SetCAFile(caFile string) {
	r.CAFile = caFile
}

func (r *RancherServerConfig) GetCAFile() string {
	return r.CAFile
}

func (r *RancherServerConfig) SetCACerts(caCerts string) {
	r.CACerts = caCerts
}

func (r *RancherServerConfig) GetCACerts() string {
	return r.CACerts
}
