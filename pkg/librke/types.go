package librke

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/rancher/rke/pki"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

type RKE interface {
	GenerateRKENodeCerts(ctx context.Context, rkeConfig v3.RancherKubernetesEngineConfig, nodeAddress string, certBundle map[string]pki.CertificatePKI) map[string]pki.CertificatePKI
	GenerateCerts(config *v3.RancherKubernetesEngineConfig) (map[string]pki.CertificatePKI, error)
	GeneratePlan(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, dockerInfo map[string]types.Info, data map[string]interface{}) (v3.RKEPlan, error)
	GenerateClusterPlan(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, dockerInfo types.Info, data map[string]interface{}) (v3.RKEClusterPlan, error)
}

func New() RKE {
	return (*rke)(nil)
}
