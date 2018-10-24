package librke

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type RKE interface {
	GenerateRKENodeCerts(ctx context.Context, rkeConfig v3.RancherKubernetesEngineConfig, nodeAddress string, certBundle map[string]pki.CertificatePKI) map[string]pki.CertificatePKI
	GenerateCerts(config *v3.RancherKubernetesEngineConfig) (map[string]pki.CertificatePKI, error)
	GeneratePlan(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, dockerInfo map[string]types.Info) (v3.RKEPlan, error)
}

func New() RKE {
	return (*rke)(nil)
}
