package clusterprovisioner

import (
	"reflect"

	"github.com/rancher/rancher/pkg/clusterprovisioninglogger"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

func (p *Provisioner) driverCreate(cluster *v3.Cluster, spec v3.ClusterSpec) (api string, token string, cert string, err error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.EventLogger, cluster, v3.ClusterConditionProvisioned)
	defer logger.Close()

	spec = cleanRKE(spec)

	if newCluster, err := p.Clusters.Update(cluster); err == nil {
		cluster = newCluster
	}

	return p.Driver.Create(ctx, cluster.Status.ClusterName, spec)
}

func (p *Provisioner) driverUpdate(cluster *v3.Cluster, spec v3.ClusterSpec) (api string, token string, cert string, err error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.EventLogger, cluster, v3.ClusterConditionUpdated)
	defer logger.Close()

	spec = cleanRKE(spec)
	applied := cleanRKE(cluster.Status.AppliedSpec)

	if spec.RancherKubernetesEngineConfig != nil && cluster.Status.APIEndpoint != "" && cluster.Status.ServiceAccountToken != "" &&
		reflect.DeepEqual(applied.RancherKubernetesEngineConfig, spec.RancherKubernetesEngineConfig) {
		return cluster.Status.APIEndpoint, cluster.Status.ServiceAccountToken, cluster.Status.CACert, nil
	}

	if newCluster, err := p.Clusters.Update(cluster); err == nil {
		cluster = newCluster
	}

	return p.Driver.Update(ctx, cluster.Status.ClusterName, spec)
}

func (p *Provisioner) driverRemove(cluster *v3.Cluster) error {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.EventLogger, cluster, v3.ClusterConditionProvisioned)
	defer logger.Close()

	spec := cleanRKE(cluster.Spec)

	_, err := v3.ClusterConditionUpdated.Do(cluster, func() (runtime.Object, error) {
		if newCluster, err := p.Clusters.Update(cluster); err == nil {
			cluster = newCluster
		}

		return cluster, p.Driver.Remove(ctx, cluster.Status.ClusterName, spec)
	})

	return err
}

func cleanRKE(spec v3.ClusterSpec) v3.ClusterSpec {
	if spec.RancherKubernetesEngineConfig == nil {
		return spec
	}

	result := spec.DeepCopy()

	var filteredNodes []v3.RKEConfigNode
	for _, node := range spec.RancherKubernetesEngineConfig.Nodes {
		if len(node.Role) == 1 && node.Role[0] == services.WorkerRole {
			continue
		}
		filteredNodes = append(filteredNodes, node)
	}

	result.RancherKubernetesEngineConfig.Nodes = filteredNodes
	return *result
}
