package clusterprovisioner

import (
	"github.com/rancher/rancher/pkg/clusterprovisioninglogger"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

func (p *Provisioner) driverCreate(cluster *v3.Cluster, spec v3.ClusterSpec) (api string, token string, cert string, err error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.EventLogger, cluster, v3.ClusterConditionProvisioned)
	defer logger.Close()

	if newCluster, err := p.Clusters.Update(cluster); err == nil {
		cluster = newCluster
	}

	if cluster.Status.Driver == v3.ClusterDriverRKE {
		return p.rke.Provision(ctx, cluster, false)
	}
	return p.Driver.Create(ctx, cluster.Status.ClusterName, spec)
}

func (p *Provisioner) driverUpdate(cluster *v3.Cluster, spec v3.ClusterSpec) (api string, token string, cert string, err error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.EventLogger, cluster, v3.ClusterConditionUpdated)
	defer logger.Close()

	if newCluster, err := p.Clusters.Update(cluster); err == nil {
		cluster = newCluster
	}

	if cluster.Status.Driver == v3.ClusterDriverRKE {
		return p.rke.Provision(ctx, cluster, true)
	}
	return p.Driver.Update(ctx, cluster.Status.ClusterName, spec)
}

func (p *Provisioner) driverRemove(cluster *v3.Cluster) error {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.EventLogger, cluster, v3.ClusterConditionProvisioned)
	defer logger.Close()

	_, err := v3.ClusterConditionUpdated.Do(cluster, func() (runtime.Object, error) {
		if newCluster, err := p.Clusters.Update(cluster); err == nil {
			cluster = newCluster
		}

		if cluster.Status.Driver == v3.ClusterDriverRKE {
			return cluster, p.rke.Remove(ctx, cluster)
		}
		return cluster, p.Driver.Remove(ctx, cluster.Status.ClusterName, cluster.Spec)
	})

	return err
}
