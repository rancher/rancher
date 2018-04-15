package librke

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type RKE interface {
	GenerateRKENodeCerts(ctx context.Context, rkeConfig v3.RancherKubernetesEngineConfig, nodeAddress string, certBundle map[string]pki.CertificatePKI) map[string]pki.CertificatePKI
	GenerateCerts(config *v3.RancherKubernetesEngineConfig) (map[string]pki.CertificatePKI, error)
	RegenerateEtcdCertificate(crtMap map[string]pki.CertificatePKI, etcdHost *hosts.Host, cluster *cluster.Cluster) (map[string]pki.CertificatePKI, error)
	ParseCluster(clusterName string, config *v3.RancherKubernetesEngineConfig, dockerDialerFactory, localConnDialerFactory hosts.DialerFactory, k8sWrapTransport k8s.WrapTransport) (*cluster.Cluster, error)
	GeneratePlan(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, dockerInfo map[string]types.Info) (v3.RKEPlan, error)
}

func New() RKE {
	return (*rke)(nil)
}
