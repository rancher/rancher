package clusterprovisioner

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/rancher/kontainer-engine/service"
	"github.com/rancher/rancher/pkg/clusterprovisioninglogger"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/runtime"
)

const DriverNameField = "driverName"

func (p *Provisioner) driverCreate(cluster *v3.Cluster, spec v3.ClusterSpec) (api string, token string, cert string, err error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, cluster, v3.ClusterConditionProvisioned)
	defer logger.Close()

	spec = cleanRKE(spec)

	if newCluster, err := p.Clusters.Update(cluster); err == nil {
		cluster = newCluster
	}

	kontainerDriver, err := p.getKontainerDriver(spec)
	if err != nil {
		return "", "", "", err
	}

	return p.Driver.Create(ctx, cluster.Name, kontainerDriver, spec)
}

func (p *Provisioner) getKontainerDriver(spec v3.ClusterSpec) (*v3.KontainerDriver, error) {
	if spec.GenericEngineConfig != nil {
		return p.KontainerDriverLister.Get("", (*spec.GenericEngineConfig)[DriverNameField].(string))
	}

	if spec.RancherKubernetesEngineConfig != nil {
		return p.KontainerDriverLister.Get("", service.RancherKubernetesEngineDriverName)
	}

	if spec.ImportedConfig != nil {
		return p.KontainerDriverLister.Get("", "import")
	}

	return nil, fmt.Errorf("no kontainer driver for cluster %v", spec.DisplayName)
}

func (p *Provisioner) driverUpdate(cluster *v3.Cluster, spec v3.ClusterSpec) (api string, token string, cert string, err error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, cluster, v3.ClusterConditionUpdated)
	defer logger.Close()

	spec = cleanRKE(spec)
	applied := cleanRKE(cluster.Status.AppliedSpec)

	if spec.RancherKubernetesEngineConfig != nil && cluster.Status.APIEndpoint != "" && cluster.Status.ServiceAccountToken != "" &&
		reflect.DeepEqual(applied.RancherKubernetesEngineConfig, spec.RancherKubernetesEngineConfig) {
		return cluster.Status.APIEndpoint, cluster.Status.ServiceAccountToken, cluster.Status.CACert, nil
	}

	if spec.RancherKubernetesEngineConfig != nil && spec.RancherKubernetesEngineConfig.Services.Etcd.Snapshot == nil &&
		applied.RancherKubernetesEngineConfig != nil && applied.RancherKubernetesEngineConfig.Services.Etcd.Snapshot == nil {
		_false := false
		cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.Snapshot = &_false
	}

	if newCluster, err := p.Clusters.Update(cluster); err == nil {
		cluster = newCluster
	}

	kontainerDriver, err := p.getKontainerDriver(spec)
	if err != nil {
		return "", "", "", err
	}

	return p.Driver.Update(ctx, cluster.Name, kontainerDriver, spec)
}

func (p *Provisioner) driverRemove(cluster *v3.Cluster) error {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, cluster, v3.ClusterConditionProvisioned)
	defer logger.Close()

	spec := cleanRKE(cluster.Spec)

	_, err := v3.ClusterConditionUpdated.Do(cluster, func() (runtime.Object, error) {
		if newCluster, err := p.Clusters.Update(cluster); err == nil {
			cluster = newCluster
		}

		kontainerDriver, err := p.getKontainerDriver(spec)
		if err != nil {
			return nil, err
		}

		return cluster, p.Driver.Remove(ctx, cluster.Name, kontainerDriver, spec)
	})

	return err
}

func (p *Provisioner) driverRestore(cluster *v3.Cluster, spec v3.ClusterSpec) (string, string, string, error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, cluster, v3.ClusterConditionUpdated)
	defer logger.Close()

	spec = cleanRKE(spec)

	newCluster, err := p.Clusters.Update(cluster)
	if err != nil {
		return "", "", "", err
	}
	cluster = newCluster

	kontainerDriver, err := p.getKontainerDriver(spec)
	if err != nil {
		return "", "", "", err
	}

	snapshot := strings.Split(spec.RancherKubernetesEngineConfig.Restore.SnapshotName, ":")[1]
	return p.Driver.ETCDRestore(ctx, cluster.Name, kontainerDriver, spec, snapshot)

}

func (p *Provisioner) generateServiceAccount(cluster *v3.Cluster, spec v3.ClusterSpec) (string, error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, cluster, v3.ClusterConditionUpdated)
	defer logger.Close()

	spec = cleanRKE(spec)

	kontainerDriver, err := p.getKontainerDriver(spec)
	if err != nil {
		return "", err
	}

	return p.Driver.GenerateServiceAccount(ctx, cluster.Name, kontainerDriver, spec)
}

func (p *Provisioner) removeLegacyServiceAccount(cluster *v3.Cluster, spec v3.ClusterSpec) error {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, cluster, v3.ClusterConditionUpdated)
	defer logger.Close()

	spec = cleanRKE(spec)

	kontainerDriver, err := p.getKontainerDriver(spec)
	if err != nil {
		return err
	}

	return p.Driver.RemoveLegacyServiceAccount(ctx, cluster.Name, kontainerDriver, spec)
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
