package clusterprovisioner

import (
	"fmt"
	"reflect"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clusterprovisioninglogger"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator/assemblers"
	"github.com/rancher/rancher/pkg/kontainer-engine/service"
	"github.com/rancher/rke/services"
	rketypes "github.com/rancher/rke/types"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const DriverNameField = "driverName"

func (p *Provisioner) driverCreate(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec) (api string, token string, cert string, err error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.ConfigMaps, cluster, apimgmtv3.ClusterConditionProvisioned)
	defer logger.Close()

	spec = cleanRKE(spec)
	spec, err = assemblers.AssembleRKEConfigSpec(cluster, spec, p.SecretLister)
	if err != nil {
		return "", "", "", err
	}

	if newCluster, err := p.Clusters.Update(cluster); err == nil {
		cluster = newCluster
	}

	kontainerDriver, err := p.getKontainerDriver(spec)
	if err != nil {
		return "", "", "", err
	}

	return p.engineService.Create(ctx, cluster.Name, kontainerDriver, spec)
}

func (p *Provisioner) getKontainerDriver(spec apimgmtv3.ClusterSpec) (*apimgmtv3.KontainerDriver, error) {
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

// driverUpdate updates the given cluster with the new config from `spec` using its driver. If `forceUpdate` is true,
// the update will be performed regardless of whether the spec has changed at all. Otherwise, the update will only
// occur if the spec has changed.
func (p *Provisioner) driverUpdate(
	cluster *apimgmtv3.Cluster,
	spec apimgmtv3.ClusterSpec,
	forceUpdate bool,
) (api string, token string, cert string, updateTriggered bool, err error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.ConfigMaps, cluster, apimgmtv3.ClusterConditionUpdated)
	defer logger.Close()

	spec = cleanRKE(spec)
	applied := cleanRKE(cluster.Status.AppliedSpec)

	configUnchanged := spec.RancherKubernetesEngineConfig != nil &&
		cluster.Status.APIEndpoint != "" &&
		cluster.Status.ServiceAccountTokenSecret != "" &&
		reflect.DeepEqual(applied.RancherKubernetesEngineConfig, spec.RancherKubernetesEngineConfig)
	if configUnchanged && !forceUpdate {
		secret, err := p.Secrets.GetNamespaced("cattle-global-data", cluster.Status.ServiceAccountTokenSecret, v1.GetOptions{})
		if err != nil {
			logrus.Errorf("Could not find service account token secret %s for cluster %s: [%v]", cluster.Status.ServiceAccountTokenSecret, cluster.Name, err)
			return cluster.Status.APIEndpoint, "", cluster.Status.CACert, false, err
		}
		return cluster.Status.APIEndpoint, string(secret.Data["credential"]), cluster.Status.CACert, false, nil
	}

	if spec.RancherKubernetesEngineConfig != nil && spec.RancherKubernetesEngineConfig.Services.Etcd.Snapshot == nil &&
		applied.RancherKubernetesEngineConfig != nil && applied.RancherKubernetesEngineConfig.Services.Etcd.Snapshot == nil {
		_false := false
		cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.Snapshot = &_false
	}

	spec, err = assemblers.AssembleRKEConfigSpec(cluster, spec, p.SecretLister)
	if err != nil {
		return "", "", "", false, err
	}

	if newCluster, err := p.Clusters.Update(cluster); err == nil {
		cluster = newCluster
	}

	kontainerDriver, err := p.getKontainerDriver(spec)
	if err != nil {
		return "", "", "", false, err
	}

	api, token, cert, err = p.engineService.Update(ctx, cluster.Name, kontainerDriver, spec)
	return api, token, cert, true, err
}

func (p *Provisioner) driverRemove(cluster *apimgmtv3.Cluster, forceRemove bool) error {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.ConfigMaps, cluster, apimgmtv3.ClusterConditionProvisioned)
	defer logger.Close()

	spec := cleanRKE(cluster.Spec)

	_, err := apimgmtv3.ClusterConditionUpdated.Do(cluster, func() (runtime.Object, error) {
		if newCluster, err := p.Clusters.Update(cluster); err == nil {
			cluster = newCluster
		}

		kontainerDriver, err := p.getKontainerDriver(spec)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Warnf("Could not find kontainer driver for cluster removal [%v]", err)
				return nil, nil
			}
			return nil, err
		}

		return cluster, p.engineService.Remove(ctx, cluster.Name, kontainerDriver, spec, forceRemove)
	})

	return err
}

func (p *Provisioner) driverRestore(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec, snapshot string) (string, string, string, error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.ConfigMaps, cluster, apimgmtv3.ClusterConditionUpdated)
	defer logger.Close()

	spec = cleanRKE(spec)
	spec, err := assemblers.AssembleRKEConfigSpec(cluster, spec, p.SecretLister)
	if err != nil {
		return "", "", "", err
	}

	newCluster, err := p.Clusters.Update(cluster)
	if err == nil {
		cluster = newCluster
	}

	kontainerDriver, err := p.getKontainerDriver(spec)
	if err != nil {
		return "", "", "", err
	}
	return p.engineService.ETCDRestore(ctx, cluster.Name, kontainerDriver, spec, snapshot)

}

func (p *Provisioner) generateServiceAccount(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec) (string, error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.ConfigMaps, cluster, apimgmtv3.ClusterConditionUpdated)
	defer logger.Close()

	spec = cleanRKE(spec)

	kontainerDriver, err := p.getKontainerDriver(spec)
	if err != nil {
		return "", err
	}

	return p.engineService.GenerateServiceAccount(ctx, cluster.Name, kontainerDriver, spec)
}

func (p *Provisioner) removeLegacyServiceAccount(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec) error {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.ConfigMaps, cluster, apimgmtv3.ClusterConditionUpdated)
	defer logger.Close()

	spec = cleanRKE(spec)

	kontainerDriver, err := p.getKontainerDriver(spec)
	if err != nil {
		return err
	}

	return p.engineService.RemoveLegacyServiceAccount(ctx, cluster.Name, kontainerDriver, spec)
}

func cleanRKE(spec apimgmtv3.ClusterSpec) apimgmtv3.ClusterSpec {
	if spec.RancherKubernetesEngineConfig == nil {
		return spec
	}

	result := spec.DeepCopy()

	var filteredNodes []rketypes.RKEConfigNode
	for _, node := range spec.RancherKubernetesEngineConfig.Nodes {
		if len(node.Role) == 1 && node.Role[0] == services.WorkerRole {
			continue
		}
		filteredNodes = append(filteredNodes, node)
	}

	result.RancherKubernetesEngineConfig.Nodes = filteredNodes
	return *result
}
