package pki

const (
	CertPathPrefix          = "/etc/kubernetes/ssl/"
	CertificatesServiceName = "certificates"
	CrtDownloaderContainer  = "cert-deployer"
	CertFetcherContainer    = "cert-fetcher"
	CertificatesSecretName  = "k8s-certs"
	TempCertPath            = "/etc/kubernetes/.tmp/"
	ClusterConfig           = "cluster.yml"

	CACertName             = "kube-ca"
	KubeAPICertName        = "kube-apiserver"
	KubeControllerCertName = "kube-controller-manager"
	KubeSchedulerCertName  = "kube-scheduler"
	KubeProxyCertName      = "kube-proxy"
	KubeNodeCertName       = "kube-node"
	EtcdCertName           = "kube-etcd"

	KubeNodeCommonName       = "system:node"
	KubeNodeOrganizationName = "system:nodes"

	KubeAdminCertName         = "kube-admin"
	KubeAdminOrganizationName = "system:masters"
	KubeAdminConfigPrefix     = "kube_config_"
)
