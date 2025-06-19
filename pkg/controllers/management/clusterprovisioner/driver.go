package clusterprovisioner

import (
	"fmt"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clusterprovisioninglogger"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

const DriverNameField = "driverName"

func (p *Provisioner) driverCreate(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec) (api string, token string, cert string, err error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.ConfigMaps, cluster, apimgmtv3.ClusterConditionProvisioned)
	defer logger.Close()

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

	if spec.ImportedConfig != nil {
		return p.KontainerDriverLister.Get("", "import")
	}

	return nil, fmt.Errorf("no kontainer driver for cluster %v", spec.DisplayName)
}

// driverUpdate updates the given cluster with the new config from `spec` using its driver
func (p *Provisioner) driverUpdate(
	cluster *apimgmtv3.Cluster,
	spec apimgmtv3.ClusterSpec,
) (api string, token string, cert string, updateTriggered bool, err error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.ConfigMaps, cluster, apimgmtv3.ClusterConditionUpdated)
	defer logger.Close()

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

	_, err := apimgmtv3.ClusterConditionUpdated.Do(cluster, func() (runtime.Object, error) {
		if newCluster, err := p.Clusters.Update(cluster); err == nil {
			cluster = newCluster
		}

		kontainerDriver, err := p.getKontainerDriver(cluster.Spec)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Warnf("Could not find kontainer driver for cluster removal [%v]", err)
				return nil, nil
			}
			return nil, err
		}

		return cluster, p.engineService.Remove(ctx, cluster.Name, kontainerDriver, cluster.Spec, forceRemove)
	})

	return err
}

func (p *Provisioner) generateServiceAccount(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec) (string, error) {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.ConfigMaps, cluster, apimgmtv3.ClusterConditionUpdated)
	defer logger.Close()

	kontainerDriver, err := p.getKontainerDriver(spec)
	if err != nil {
		return "", err
	}

	return p.engineService.GenerateServiceAccount(ctx, cluster.Name, kontainerDriver, spec)
}

func (p *Provisioner) removeLegacyServiceAccount(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec) error {
	ctx, logger := clusterprovisioninglogger.NewLogger(p.Clusters, p.ConfigMaps, cluster, apimgmtv3.ClusterConditionUpdated)
	defer logger.Close()

	kontainerDriver, err := p.getKontainerDriver(spec)
	if err != nil {
		return err
	}

	return p.engineService.RemoveLegacyServiceAccount(ctx, cluster.Name, kontainerDriver, spec)
}
