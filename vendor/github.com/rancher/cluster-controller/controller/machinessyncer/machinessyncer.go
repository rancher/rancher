package machinessyncer

import (
	"fmt"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	finalizerName = "machinesSyncer"
)

type Syncer struct {
	Machines v3.MachineInterface
	Clusters v3.ClusterInterface
}

func Register(management *config.ManagementContext) {
	s := &Syncer{
		Machines: management.Management.Machines(""),
		Clusters: management.Management.Clusters(""),
	}
	management.Management.Machines("").Controller().AddHandler(s.sync)
}

func (s *Syncer) sync(key string, machine *v3.Machine) error {
	if machine == nil {
		return nil
	}
	if machine.DeletionTimestamp != nil {
		return s.removeFromClusterConfig(machine)
	}
	return s.addToClusterConfig(machine)
}

func getClusterName(machine *v3.Machine) string {
	clusterName := machine.ClusterName
	if clusterName == "" {
		clusterName = machine.Spec.ClusterName
	}
	return clusterName
}

func (s *Syncer) addToClusterConfig(machine *v3.Machine) error {
	clusterName := getClusterName(machine)
	if clusterName == "" {
		return nil
	}
	if machine.Spec.MachineTemplateName == "" {
		// regular, non machine provisioned host
		return nil
	}

	if machine.Status.NodeConfig == nil {
		logrus.Debugf("Machine node [%s] for cluster [%s] is not provisioned yet", machine.Name, clusterName)
		return nil
	}

	cluster, err := s.Clusters.Get(clusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if cluster == nil {
		return fmt.Errorf("Cluster [%s] does not exist", clusterName)
	}

	if cluster == nil || cluster.Spec.RancherKubernetesEngineConfig == nil {
		return fmt.Errorf("Cluster [%s] can not accept nodes provisioned by machine", clusterName)
	}

	// 1. update machine finalizer
	set := finalizerSet(machine)
	machineToUpdate := machine.DeepCopy()
	if !set {
		machineToUpdate.Finalizers = append(machineToUpdate.Finalizers, finalizerName)
	}
	_, err = s.Machines.Update(machineToUpdate)
	if err != nil {
		return fmt.Errorf("Failed to update machine [%s] finalizers in cluster [%s]: %v",
			machine.Name, cluster.Name, err)
	}

	var updatedNodes []v3.RKEConfigNode
	needToAdd := true
	for _, node := range cluster.Spec.RancherKubernetesEngineConfig.Nodes {
		if node.MachineName == machine.Name {
			updatedNodes = append(updatedNodes, *machine.Status.NodeConfig)
			needToAdd = false
			continue
		}
		updatedNodes = append(updatedNodes, node)
	}
	if needToAdd {
		updatedNodes = append(updatedNodes, *machine.Status.NodeConfig)
	}
	cluster.Spec.RancherKubernetesEngineConfig.Nodes = updatedNodes
	_, err = s.Clusters.Update(cluster)
	if err != nil {
		return err
	}

	return nil
}

func (s *Syncer) removeFromClusterConfig(machine *v3.Machine) error {
	if len(machine.Finalizers) <= 0 || machine.Finalizers[0] != finalizerName {
		return nil
	}
	cluster, err := s.Clusters.Get(getClusterName(machine), metav1.GetOptions{})
	if err != nil {
		return err
	}
	if cluster == nil || cluster.Spec.RancherKubernetesEngineConfig == nil {
		s.cleanupFinalizer(machine)
		return nil
	}

	var updatedNodes []v3.RKEConfigNode
	updated := false
	for _, node := range cluster.Spec.RancherKubernetesEngineConfig.Nodes {
		if node.MachineName == machine.Name {
			logrus.Infof("Removing machine [%s] from cluster [%s]", machine.Name, cluster.Name)
			updated = true
			continue
		}
		updatedNodes = append(updatedNodes, node)
	}
	if updated {
		cluster.Spec.RancherKubernetesEngineConfig.Nodes = updatedNodes
		_, err := s.Clusters.Update(cluster)
		if err != nil {
			return err
		}
	}

	s.cleanupFinalizer(machine)

	return nil
}

func (s *Syncer) cleanupFinalizer(machine *v3.Machine) error {
	toUpdate := machine.DeepCopy()
	var finalizers []string
	for _, finalizer := range machine.Finalizers {
		if finalizer == finalizerName {
			continue
		}
		finalizers = append(finalizers, finalizer)
	}
	toUpdate.Finalizers = finalizers
	_, err := s.Machines.Update(toUpdate)
	return err
}

func finalizerSet(machine *v3.Machine) bool {
	for _, value := range machine.Finalizers {
		if value == finalizerName {
			return true
		}
	}
	return false
}
