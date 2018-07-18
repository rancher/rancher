package pki

const (
	CertPathPrefix          = "/etc/kubernetes/ssl/"
	CertificatesServiceName = "certificates"
	CrtDownloaderContainer  = "cert-deployer"
	CertFetcherContainer    = "cert-fetcher"
	CertificatesSecretName  = "k8s-certs"
	TempCertPath            = "/etc/kubernetes/.tmp/"
	ClusterConfig           = "cluster.yml"
	BundleCertPath          = "/backup/pki.bundle.tar.gz"

	CACertName              = "kube-ca"
	RequestHeaderCACertName = "kube-apiserver-requestheader-ca"
	KubeAPICertName         = "kube-apiserver"
	KubeControllerCertName  = "kube-controller-manager"
	KubeSchedulerCertName   = "kube-scheduler"
	KubeProxyCertName       = "kube-proxy"
	KubeNodeCertName        = "kube-node"
	EtcdCertName            = "kube-etcd"
	EtcdClientCACertName    = "kube-etcd-client-ca"
	EtcdClientCertName      = "kube-etcd-client"
	APIProxyClientCertName  = "kube-apiserver-proxy-client"

	KubeNodeCommonName       = "system:node"
	KubeNodeOrganizationName = "system:nodes"

	KubeAdminCertName         = "kube-admin"
	KubeAdminOrganizationName = "system:masters"
	KubeAdminConfigPrefix     = "kube_config_"
)
