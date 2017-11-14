package clusterprovisioner

import (
	"fmt"
	"reflect"
	"time"

	driver "github.com/rancher/kontainer-engine/stub"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	p := &Provisioner{
		Clusters: management.Management.Clusters(""),
	}
	management.Management.Clusters("").Controller().AddHandler(p.sync)
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

func getAction(cluster *v3.Cluster) string {
	if cluster == nil {
		return NoopAction
	}
	if cluster.ObjectMeta.DeletionTimestamp != nil {
		return RemoveAction
	}

	if !isClusterProvisioned(cluster) {
		return CreateAction
	}

	if configChanged(cluster) {
		return UpdateAction
	}
	return NoopAction
}

func (p *Provisioner) sync(key string, cluster *v3.Cluster) error {
	action := getAction(cluster)
	switch action {
	case CreateAction:
		return p.createCluster(cluster)
	case UpdateAction:
		return p.updateCluster(cluster)
	case RemoveAction:
		return p.removeCluster(cluster)
	default:
		return nil
	}
}

func (p *Provisioner) removeCluster(cluster *v3.Cluster) error {
	set, index := p.finalizerSet(cluster)
	if set && index == 0 {
		logrus.Infof("Deleting cluster [%s]", cluster.Name)
		// 1. Call the driver to remove the cluster
		if needToProvision(cluster) && isClusterProvisioned(cluster) {
			for i := 0; i < 4; i++ {
				err := driver.Remove(cluster.Name, cluster.Spec)
				if err == nil {
					break
				}
				if i == 3 {
					return fmt.Errorf("Failed to remove the cluster [%s]: %v", cluster.Name, err)
				}
				time.Sleep(1 * time.Second)
			}
		}

		// 2. Remove the finalizer
		toUpdate := cluster.DeepCopy()
		var finalizers []string
		for _, finalizer := range cluster.Finalizers {
			if finalizer == p.GetName() {
				continue
			}
			finalizers = append(finalizers, finalizer)
		}
		toUpdate.Finalizers = finalizers
		_, err := p.Clusters.Update(toUpdate)
		if err != nil {
			p.Clusters.Delete(toUpdate.Name, nil)
			return fmt.Errorf("Failed to reset finalizers for cluster [%s]: %v", cluster.Name, err)
		}
		logrus.Infof("Deleted cluster [%s]", cluster.Name)
	}

	return nil
}

func (p *Provisioner) updateCluster(cluster *v3.Cluster) error {
	err := p.preUpdateClusterStatus(cluster.Name)
	if err != nil {
		return fmt.Errorf("Failed to update status for cluster [%s]: %v", cluster.Name, err)
	}
	logrus.Infof("Updating cluster [%s]", cluster.Name)
	var apiEndpoint, serviceAccountToken, caCert string
	if needToProvision(cluster) {
		apiEndpoint, serviceAccountToken, caCert, err = driver.Update(cluster.Name, cluster.Spec)
		if err != nil {
			_ = p.postUpdateClusterStatusError(cluster, err)
			return fmt.Errorf("Failed to update the cluster [%s]: %v", cluster.Name, err)
		}
	}

	err = p.postUpdateClusterStatusSuccess(cluster, apiEndpoint, serviceAccountToken, caCert)
	if err != nil {
		return fmt.Errorf("Failed to update status for cluster [%s]: %v", cluster.Name, err)
	}
	logrus.Infof("Updated cluster [%s]", cluster.Name)
	return nil
}

func (p *Provisioner) createCluster(cluster *v3.Cluster) error {
	err := p.preUpdateClusterStatus(cluster.Name)
	if err != nil {
		return fmt.Errorf("Failed to update status for cluster [%s]: %v", cluster.Name, err)
	}
	logrus.Infof("Provisioning cluster [%s]", cluster.Name)

	var apiEndpoint, serviceAccountToken, caCert string
	if needToProvision(cluster) {
		apiEndpoint, serviceAccountToken, caCert, err = driver.Create(cluster.Name, cluster.Spec)
		if err != nil {
			_ = p.postUpdateClusterStatusError(cluster, err)
			return fmt.Errorf("Failed to provision the cluster [%s]: %v", cluster.Name, err)
		}
	}

	err = p.postUpdateClusterStatusSuccess(cluster, apiEndpoint, serviceAccountToken, caCert)
	if err != nil {
		return fmt.Errorf("Failed to update status for cluster [%s]: %v", cluster.Name, err)
	}
	logrus.Infof("Provisioned cluster [%s]", cluster.Name)
	return nil
}

func (p *Provisioner) GetName() string {
	return "clusterProvisioner"
}

func (p *Provisioner) postUpdateClusterStatusError(cluster *v3.Cluster, userError error) error {
	toUpdate, err := p.Clusters.Get(cluster.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	condition := newClusterCondition(v3.ClusterConditionUpdating, "True", fmt.Sprintf("Failed to update cluster %s", userError.Error()))
	setClusterCondition(&toUpdate.Status, condition)
	_, err = p.Clusters.Update(toUpdate)
	return err
}

func (p *Provisioner) postUpdateClusterStatusSuccess(cluster *v3.Cluster, apiEndpiont string, serviceAccountToken string, caCert string) error {
	toUpdate, err := p.Clusters.Get(cluster.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	toUpdate.Status.AppliedSpec = cluster.Spec
	toUpdate.Status.APIEndpoint = apiEndpiont
	toUpdate.Status.ServiceAccountToken = serviceAccountToken
	toUpdate.Status.CACert = caCert
	if !isClusterProvisioned(cluster) {
		condition := newClusterCondition(v3.ClusterConditionProvisioned, "True", "Cluster provisioned successfully")
		setClusterCondition(&toUpdate.Status, condition)
	}

	condition := newClusterCondition(v3.ClusterConditionUpdating, "False", "Cluster updated successfully")
	setClusterCondition(&toUpdate.Status, condition)
	_, err = p.Clusters.Update(toUpdate)
	return err
}

func newClusterCondition(condType v3.ClusterConditionType, status v1.ConditionStatus, reason string) v3.ClusterCondition {
	now := time.Now().Format(time.RFC3339)
	return v3.ClusterCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     now,
		LastTransitionTime: now,
		Reason:             reason,
	}
}

func (p *Provisioner) preUpdateClusterStatus(clusterName string) error {
	toUpdate, err := p.Clusters.Get(clusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if toUpdate.Status.Conditions == nil {
		// init conditions
		conditions := []v3.ClusterCondition{}
		conditions = append(conditions, newClusterCondition(v3.ClusterConditionNoMemoryPressure, "Unknown", ""))
		conditions = append(conditions, newClusterCondition(v3.ClusterConditionNoDiskPressure, "Unknown", ""))
		conditions = append(conditions, newClusterCondition(v3.ClusterConditionReady, "Unknown", ""))
		conditions = append(conditions, newClusterCondition(v3.ClusterConditionUpdating, "True", ""))
		conditions = append(conditions, newClusterCondition(v3.ClusterConditionProvisioned, "False", ""))
		toUpdate.Status.Conditions = conditions
		toUpdate.Status.ComponentStatuses = []v3.ClusterComponentStatus{}
	} else {
		condition := newClusterCondition(v3.ClusterConditionUpdating, "True", "")
		setClusterCondition(&toUpdate.Status, condition)
	}

	set, _ := p.finalizerSet(toUpdate)

	if !set {
		toUpdate.ObjectMeta.Finalizers = append(toUpdate.ObjectMeta.Finalizers, p.GetName())
	}
	_, err = p.Clusters.Update(toUpdate)
	return err
}

func (p *Provisioner) finalizerSet(cluster *v3.Cluster) (bool, int) {
	i := 0
	for _, value := range cluster.ObjectMeta.Finalizers {
		if value == p.GetName() {
			return true, i
		}
		i++
	}
	return false, -1
}

func setClusterCondition(status *v3.ClusterStatus, c v3.ClusterCondition) {
	pos, cp := getClusterCondition(status, c.Type)
	if cp != nil && cp.Status == c.Status {
		return
	}

	if cp != nil {
		status.Conditions[pos] = c
	} else {
		status.Conditions = append(status.Conditions, c)
	}
}

func getClusterCondition(status *v3.ClusterStatus, t v3.ClusterConditionType) (int, *v3.ClusterCondition) {
	for i, c := range status.Conditions {
		if t == c.Type {
			return i, &c
		}
	}
	return -1, nil
}

func isClusterProvisioned(cluster *v3.Cluster) bool {
	_, isProvisioned := getClusterCondition(&cluster.Status, v3.ClusterConditionProvisioned)
	if isProvisioned == nil {
		return false
	}
	return isProvisioned.Status == "True"
}

func needToProvision(cluster *v3.Cluster) bool {
	return cluster.Spec.RancherKubernetesEngineConfig != nil || cluster.Spec.AzureKubernetesServiceConfig != nil || cluster.Spec.GoogleKubernetesEngineConfig != nil
}
