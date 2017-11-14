package cluster

import (
	"fmt"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/services"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

func ReconcileCluster(kubeCluster, currentCluster *Cluster) error {
	logrus.Infof("[reconcile] Reconciling cluster state")
	if currentCluster == nil {
		logrus.Infof("[reconcile] This is newly generated cluster")

		return nil
	}
	kubeClient, err := k8s.NewClient(kubeCluster.LocalKubeConfigPath)
	if err != nil {
		return fmt.Errorf("Failed to initialize new kubernetes client: %v", err)
	}

	if err := reconcileWorker(currentCluster, kubeCluster, kubeClient); err != nil {
		return err
	}

	if err := reconcileControl(currentCluster, kubeCluster, kubeClient); err != nil {
		return err
	}
	logrus.Infof("[reconcile] Reconciled cluster state successfully")
	return nil
}

func reconcileWorker(currentCluster, kubeCluster *Cluster, kubeClient *kubernetes.Clientset) error {
	// worker deleted first to avoid issues when worker+controller on same host
	logrus.Debugf("[reconcile] Check worker hosts to be deleted")
	wpToDelete := hosts.GetToDeleteHosts(currentCluster.WorkerHosts, kubeCluster.WorkerHosts)
	for _, toDeleteHost := range wpToDelete {
		toDeleteHost.IsWorker = false
		if err := hosts.DeleteNode(toDeleteHost, kubeClient, toDeleteHost.IsControl); err != nil {
			return fmt.Errorf("Failed to delete worker node %s from cluster", toDeleteHost.Address)
		}
		// attempting to clean services/files on the host
		if err := reconcileHost(toDeleteHost, true, currentCluster.SystemImages[AplineImage]); err != nil {
			logrus.Warnf("[reconcile] Couldn't clean up worker node [%s]: %v", toDeleteHost.Address, err)
			continue
		}
	}
	return nil
}

func reconcileControl(currentCluster, kubeCluster *Cluster, kubeClient *kubernetes.Clientset) error {
	logrus.Debugf("[reconcile] Check Control plane hosts to be deleted")
	selfDeleteAddress, err := getLocalConfigAddress(kubeCluster.LocalKubeConfigPath)
	if err != nil {
		return err
	}
	cpToDelete := hosts.GetToDeleteHosts(currentCluster.ControlPlaneHosts, kubeCluster.ControlPlaneHosts)
	// move the current host in local kubeconfig to the end of the list
	for i, toDeleteHost := range cpToDelete {
		if toDeleteHost.Address == selfDeleteAddress {
			cpToDelete = append(cpToDelete[:i], cpToDelete[i+1:]...)
			cpToDelete = append(cpToDelete, toDeleteHost)
		}
	}

	for _, toDeleteHost := range cpToDelete {
		kubeClient, err := k8s.NewClient(kubeCluster.LocalKubeConfigPath)
		if err != nil {
			return fmt.Errorf("Failed to initialize new kubernetes client: %v", err)
		}
		if err := hosts.DeleteNode(toDeleteHost, kubeClient, toDeleteHost.IsWorker); err != nil {
			return fmt.Errorf("Failed to delete controlplane node %s from cluster", toDeleteHost.Address)
		}
		// attempting to clean services/files on the host
		if err := reconcileHost(toDeleteHost, false, currentCluster.SystemImages[AplineImage]); err != nil {
			logrus.Warnf("[reconcile] Couldn't clean up controlplane node [%s]: %v", toDeleteHost.Address, err)
			continue
		}
	}
	// rebuilding local admin config to enable saving cluster state
	if err := rebuildLocalAdminConfig(kubeCluster); err != nil {
		return err
	}
	// Rolling update on change for nginx Proxy
	cpChanged := hosts.IsHostListChanged(currentCluster.ControlPlaneHosts, kubeCluster.ControlPlaneHosts)
	if cpChanged {
		logrus.Infof("[reconcile] Rolling update nginx hosts with new list of control plane hosts")
		err := services.RollingUpdateNginxProxy(kubeCluster.ControlPlaneHosts, kubeCluster.WorkerHosts, currentCluster.SystemImages[NginxProxyImage])
		if err != nil {
			return fmt.Errorf("Failed to rolling update Nginx hosts with new control plane hosts")
		}
	}
	return nil
}

func reconcileHost(toDeleteHost *hosts.Host, worker bool, cleanerImage string) error {
	if err := toDeleteHost.TunnelUp(); err != nil {
		return fmt.Errorf("Not able to reach the host: %v", err)
	}
	if worker {
		if err := services.RemoveWorkerPlane([]*hosts.Host{toDeleteHost}, false); err != nil {
			return fmt.Errorf("Couldn't remove worker plane: %v", err)
		}
		if err := toDeleteHost.CleanUpWorkerHost(services.ControlRole, cleanerImage); err != nil {
			return fmt.Errorf("Not able to clean the host: %v", err)
		}
	} else {
		if err := services.RemoveControlPlane([]*hosts.Host{toDeleteHost}, false); err != nil {
			return fmt.Errorf("Couldn't remove control plane: %v", err)
		}
		if err := toDeleteHost.CleanUpControlHost(services.WorkerRole, cleanerImage); err != nil {
			return fmt.Errorf("Not able to clean the host: %v", err)
		}
	}
	return nil
}
