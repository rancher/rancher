package clusterprovisioner

import (
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	driver "github.com/rancher/kontainer-engine/stub"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	RemoveAction = "Remove"
	UpdateAction = "Update"
	CreateAction = "Create"
	NoopAction   = "Noop"
)

type Provisioner struct {
	Clusters v3.ClusterInterface
}

func Register(management *config.ManagementContext) {
	clustersClient := management.Management.Clusters("")
	p := &Provisioner{
		Clusters: clustersClient,
	}
	clustersClient.AddLifecycle(p.GetName(), p)
}

func configChanged(cluster *v3.Cluster) bool {
	changed := false
	if cluster.Spec.AzureKubernetesServiceConfig != nil {
		applied := cluster.Status.AppliedSpec.AzureKubernetesServiceConfig
		current := cluster.Spec.AzureKubernetesServiceConfig
		changed = applied != nil && !reflect.DeepEqual(applied, current)
	} else if cluster.Spec.GoogleKubernetesEngineConfig != nil {
		applied := cluster.Status.AppliedSpec.GoogleKubernetesEngineConfig
		current := cluster.Spec.GoogleKubernetesEngineConfig
		changed = applied != nil && !reflect.DeepEqual(applied, current)
	} else if cluster.Spec.RancherKubernetesEngineConfig != nil {
		applied := cluster.Status.AppliedSpec.RancherKubernetesEngineConfig
		current := cluster.Spec.RancherKubernetesEngineConfig
		changed = applied != nil && !reflect.DeepEqual(applied, current)
	}

	return changed
}

func (p *Provisioner) Remove(cluster *v3.Cluster) (*v3.Cluster, error) {
	logrus.Infof("Deleting cluster [%s]", cluster.Name)
	if needToProvision(cluster) && v3.ClusterConditionProvisioned.IsTrue(cluster) {
		for i := 0; i < 4; i++ {
			err := driver.Remove(cluster.Name, cluster.Spec)
			if err == nil {
				break
			}
			if i == 3 {
				return cluster, fmt.Errorf("Failed to remove the cluster [%s]: %v", cluster.Name, err)
			}
			time.Sleep(1 * time.Second)
		}
	}
	logrus.Infof("Deleted cluster [%s]", cluster.Name)

	return cluster, nil
}

func (p *Provisioner) Updated(cluster *v3.Cluster) (*v3.Cluster, error) {
	if v3.ClusterConditionProvisioned.IsTrue(cluster) && configChanged(cluster) {
		return p.reconcileCluster(cluster)
	}
	return cluster, nil
}

func (p *Provisioner) Create(cluster *v3.Cluster) (*v3.Cluster, error) {
	if v3.ClusterConditionProvisioned.IsTrue(cluster) {
		return cluster, nil
	}
	return p.reconcileCluster(cluster)
}

func (p *Provisioner) reconcileCluster(cluster *v3.Cluster) (*v3.Cluster, error) {
	newObj, err := v3.ClusterConditionProvisioned.Do(cluster, func() (runtime.Object, error) {
		if needToProvision(cluster) {
			logrus.Infof("Provisioning cluster [%s]", cluster.Name)
			apiEndpoint, serviceAccountToken, caCert, err := driver.Update(cluster.Name, cluster.Spec)
			if err != nil {
				return cluster, errors.Wrapf(err, "Failed to provision cluster [%s]", cluster.Name)
			}
			cluster.Status.AppliedSpec = cluster.Spec
			cluster.Status.APIEndpoint = apiEndpoint
			cluster.Status.ServiceAccountToken = serviceAccountToken
			cluster.Status.CACert = caCert
			logrus.Infof("Provisioned cluster [%s]", cluster.Name)
		}
		return cluster, nil
	})

	return newObj.(*v3.Cluster), err
}

func (p *Provisioner) GetName() string {
	return "cluster-provisioner-controller"
}

func needToProvision(cluster *v3.Cluster) bool {
	return cluster.Spec.RancherKubernetesEngineConfig != nil || cluster.Spec.AzureKubernetesServiceConfig != nil || cluster.Spec.GoogleKubernetesEngineConfig != nil
}
