package librke

import (
	"context"

	rketypes "github.com/rancher/rke/types"

	"github.com/docker/docker/api/types"
	"github.com/rancher/rke/pki"
)

type RKE interface {
	GenerateRKENodeCerts(ctx context.Context, rkeConfig rketypes.RancherKubernetesEngineConfig, nodeAddress string, certBundle map[string]pki.CertificatePKI) map[string]pki.CertificatePKI
	GenerateCerts(config *rketypes.RancherKubernetesEngineConfig) (map[string]pki.CertificatePKI, error)
	GeneratePlan(ctx context.Context, rkeConfig *rketypes.RancherKubernetesEngineConfig, dockerInfo map[string]types.Info, data map[string]interface{}) (rketypes.RKEPlan, error)
}

func New() RKE {
	return (*rke)(nil)
}
