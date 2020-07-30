package pki

import "time"

const (
	K8sBaseDir              = "/etc/kubernetes/"
	CertPathPrefix          = K8sBaseDir + "ssl/"
	CertificatesServiceName = "certificates"
	CrtDownloaderContainer  = "cert-deployer"
	CertFetcherContainer    = "cert-fetcher"
	CertificatesSecretName  = "k8s-certs"
	TempCertPath            = "/etc/kubernetes/.tmp/"
	ClusterConfig           = "cluster.yml"
	ClusterStateFile        = "cluster-state.yml"
	ClusterStateExt         = ".rkestate"
	ClusterStateEnv         = "CLUSTER_STATE"
	BundleCertPath          = "/backup/pki.bundle.tar.gz"

	CACertName                 = "kube-ca"
	RequestHeaderCACertName    = "kube-apiserver-requestheader-ca"
	KubeAPICertName            = "kube-apiserver"
	KubeControllerCertName     = "kube-controller-manager"
	KubeSchedulerCertName      = "kube-scheduler"
	KubeProxyCertName          = "kube-proxy"
	KubeNodeCertName           = "kube-node"
	KubeletCertName            = "kube-kubelet"
	EtcdCertName               = "kube-etcd"
	EtcdClientCACertName       = "kube-etcd-client-ca"
	EtcdClientCertName         = "kube-etcd-client"
	APIProxyClientCertName     = "kube-apiserver-proxy-client"
	ServiceAccountTokenKeyName = "kube-service-account-token"

	KubeNodeCommonName       = "system:node"
	KubeNodeOrganizationName = "system:nodes"

	KubeAdminCertName         = "kube-admin"
	KubeAdminOrganizationName = "system:masters"
	KubeAdminConfigPrefix     = "kube_config_"
	duration365d              = time.Hour * 24 * 365
)
