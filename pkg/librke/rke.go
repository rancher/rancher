package librke

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/pki"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

type rke struct {
}

func (*rke) GenerateRKENodeCerts(ctx context.Context, rkeConfig v3.RancherKubernetesEngineConfig, nodeAddress string, certBundle map[string]pki.CertificatePKI) map[string]pki.CertificatePKI {
	return pki.GenerateRKENodeCerts(ctx, rkeConfig, nodeAddress, certBundle)
}

func (*rke) GenerateCerts(config *v3.RancherKubernetesEngineConfig) (map[string]pki.CertificatePKI, error) {
	return pki.GenerateRKECerts(context.Background(), *config, "", "")
}

func (*rke) GeneratePlan(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, dockerInfo map[string]types.Info, data map[string]interface{}) (v3.RKEPlan, error) {
	return cluster.GeneratePlan(ctx, rkeConfig.DeepCopy(), dockerInfo, data)
}

func (*rke) GenerateClusterPlan(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, dockerInfo types.Info, data map[string]interface{}) (v3.RKEClusterPlan, error) {
	return cluster.GenerateClusterPlan(ctx, rkeConfig.DeepCopy(), dockerInfo, data)
}

func GetDockerInfo(node *v3.Node) (map[string]types.Info, error) {
	infos := map[string]types.Info{}
	if node.Status.DockerInfo != nil {
		dockerInfo := types.Info{}
		err := convert.ToObj(node.Status.DockerInfo, &dockerInfo)
		if err != nil {
			return nil, err
		}
		infos[node.Status.NodeConfig.Address] = dockerInfo
	}

	return infos, nil
}
