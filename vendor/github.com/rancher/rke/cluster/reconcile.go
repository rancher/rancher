package cluster

import (
	"context"
	"fmt"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/cert"
)

const (
	unschedulableEtcdTaint = "node-role.kubernetes.io/etcd=true:NoExecute"
)

func ReconcileCluster(ctx context.Context, kubeCluster, currentCluster *Cluster) error {
	log.Infof(ctx, "[reconcile] Reconciling cluster state")
	if currentCluster == nil {
		log.Infof(ctx, "[reconcile] This is newly generated cluster")

		return nil
	}

	kubeClient, err := k8s.NewClient(kubeCluster.LocalKubeConfigPath)
	if err != nil {
		return fmt.Errorf("Failed to initialize new kubernetes client: %v", err)
	}
	// sync node labels to define the toDelete labels
	syncLabels(ctx, currentCluster, kubeCluster)

	if err := reconcileEtcd(ctx, currentCluster, kubeCluster, kubeClient); err != nil {
		return fmt.Errorf("Failed to reconcile etcd plane: %v", err)
	}

	if err := reconcileWorker(ctx, currentCluster, kubeCluster, kubeClient); err != nil {
		return err
	}

	if err := reconcileControl(ctx, currentCluster, kubeCluster, kubeClient); err != nil {
		return err
	}
	log.Infof(ctx, "[reconcile] Reconciled cluster state successfully")
	return nil
}

func reconcileWorker(ctx context.Context, currentCluster, kubeCluster *Cluster, kubeClient *kubernetes.Clientset) error {
	// worker deleted first to avoid issues when worker+controller on same host
	logrus.Debugf("[reconcile] Check worker hosts to be deleted")
	wpToDelete := hosts.GetToDeleteHosts(currentCluster.WorkerHosts, kubeCluster.WorkerHosts)
	for _, toDeleteHost := range wpToDelete {
		toDeleteHost.IsWorker = false
		if err := hosts.DeleteNode(ctx, toDeleteHost, kubeClient, toDeleteHost.IsControl); err != nil {
			return fmt.Errorf("Failed to delete worker node %s from cluster", toDeleteHost.Address)
		}
		// attempting to clean services/files on the host
		if err := reconcileHost(ctx, toDeleteHost, true, false, currentCluster.SystemImages.Alpine, currentCluster.DockerDialerFactory, currentCluster.PrivateRegistriesMap); err != nil {
			log.Warnf(ctx, "[reconcile] Couldn't clean up worker node [%s]: %v", toDeleteHost.Address, err)
			continue
		}
	}
	// attempt to remove unschedulable taint
	toAddHosts := hosts.GetToAddHosts(currentCluster.WorkerHosts, kubeCluster.WorkerHosts)
	for _, host := range toAddHosts {
		if host.IsEtcd {
			host.ToDelTaints = append(host.ToDelTaints, unschedulableEtcdTaint)
		}
	}
	return nil
}

func reconcileControl(ctx context.Context, currentCluster, kubeCluster *Cluster, kubeClient *kubernetes.Clientset) error {
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
		if err := hosts.DeleteNode(ctx, toDeleteHost, kubeClient, toDeleteHost.IsWorker); err != nil {
			return fmt.Errorf("Failed to delete controlplane node %s from cluster", toDeleteHost.Address)
		}
		// attempting to clean services/files on the host
		if err := reconcileHost(ctx, toDeleteHost, false, false, currentCluster.SystemImages.Alpine, currentCluster.DockerDialerFactory, currentCluster.PrivateRegistriesMap); err != nil {
			log.Warnf(ctx, "[reconcile] Couldn't clean up controlplane node [%s]: %v", toDeleteHost.Address, err)
			continue
		}
	}
	// rebuilding local admin config to enable saving cluster state
	if err := rebuildLocalAdminConfig(ctx, kubeCluster); err != nil {
		return err
	}
	// attempt to remove unschedulable taint
	toAddHosts := hosts.GetToAddHosts(currentCluster.ControlPlaneHosts, kubeCluster.ControlPlaneHosts)
	for _, host := range toAddHosts {
		if host.IsEtcd {
			host.ToDelTaints = append(host.ToDelTaints, unschedulableEtcdTaint)
		}
	}
	return nil
}

func reconcileHost(ctx context.Context, toDeleteHost *hosts.Host, worker, etcd bool, cleanerImage string, dialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry) error {
	if err := toDeleteHost.TunnelUp(ctx, dialerFactory); err != nil {
		return fmt.Errorf("Not able to reach the host: %v", err)
	}
	if worker {
		if err := services.RemoveWorkerPlane(ctx, []*hosts.Host{toDeleteHost}, false); err != nil {
			return fmt.Errorf("Couldn't remove worker plane: %v", err)
		}
		if err := toDeleteHost.CleanUpWorkerHost(ctx, cleanerImage, prsMap); err != nil {
			return fmt.Errorf("Not able to clean the host: %v", err)
		}
	} else if etcd {
		if err := services.RemoveEtcdPlane(ctx, []*hosts.Host{toDeleteHost}, false); err != nil {
			return fmt.Errorf("Couldn't remove etcd plane: %v", err)
		}
		if err := toDeleteHost.CleanUpEtcdHost(ctx, cleanerImage, prsMap); err != nil {
			return fmt.Errorf("Not able to clean the host: %v", err)
		}
	} else {
		if err := services.RemoveControlPlane(ctx, []*hosts.Host{toDeleteHost}, false); err != nil {
			return fmt.Errorf("Couldn't remove control plane: %v", err)
		}
		if err := toDeleteHost.CleanUpControlHost(ctx, cleanerImage, prsMap); err != nil {
			return fmt.Errorf("Not able to clean the host: %v", err)
		}
	}
	return nil
}

func reconcileEtcd(ctx context.Context, currentCluster, kubeCluster *Cluster, kubeClient *kubernetes.Clientset) error {
	log.Infof(ctx, "[reconcile] Check etcd hosts to be deleted")
	// get tls for the first current etcd host
	clientCert := cert.EncodeCertPEM(currentCluster.Certificates[pki.KubeNodeCertName].Certificate)
	clientkey := cert.EncodePrivateKeyPEM(currentCluster.Certificates[pki.KubeNodeCertName].Key)

	etcdToDelete := hosts.GetToDeleteHosts(currentCluster.EtcdHosts, kubeCluster.EtcdHosts)
	for _, etcdHost := range etcdToDelete {
		if err := services.RemoveEtcdMember(ctx, etcdHost, kubeCluster.EtcdHosts, currentCluster.LocalConnDialerFactory, clientCert, clientkey); err != nil {
			log.Warnf(ctx, "[reconcile] %v", err)
			continue
		}
		if err := hosts.DeleteNode(ctx, etcdHost, kubeClient, etcdHost.IsControl); err != nil {
			log.Warnf(ctx, "Failed to delete etcd node %s from cluster", etcdHost.Address)
			continue
		}
		// attempting to clean services/files on the host
		if err := reconcileHost(ctx, etcdHost, false, true, currentCluster.SystemImages.Alpine, currentCluster.DockerDialerFactory, currentCluster.PrivateRegistriesMap); err != nil {
			log.Warnf(ctx, "[reconcile] Couldn't clean up etcd node [%s]: %v", etcdHost.Address, err)
			continue
		}
	}
	log.Infof(ctx, "[reconcile] Check etcd hosts to be added")
	etcdToAdd := hosts.GetToAddHosts(currentCluster.EtcdHosts, kubeCluster.EtcdHosts)
	crtMap := currentCluster.Certificates
	var err error
	for _, etcdHost := range etcdToAdd {
		etcdHost.ToAddEtcdMember = true
		// Generate new certificate for the new etcd member
		crtMap, err = pki.RegenerateEtcdCertificate(
			ctx,
			crtMap,
			etcdHost,
			kubeCluster.EtcdHosts,
			kubeCluster.ClusterDomain,
			kubeCluster.KubernetesServiceIP)
		if err != nil {
			return err
		}
	}
	currentCluster.Certificates = crtMap
	for _, etcdHost := range etcdToAdd {
		// deploy certificates on new etcd host
		if err := pki.DeployCertificatesOnHost(ctx, etcdHost, currentCluster.Certificates, kubeCluster.SystemImages.CertDownloader, pki.CertPathPrefix, kubeCluster.PrivateRegistriesMap); err != nil {
			return err
		}

		if err := services.AddEtcdMember(ctx, etcdHost, kubeCluster.EtcdHosts, currentCluster.LocalConnDialerFactory, clientCert, clientkey); err != nil {
			return err
		}
		etcdHost.ToAddEtcdMember = false
		readyHosts := getReadyEtcdHosts(kubeCluster.EtcdHosts)
		etcdProcessHostMap := kubeCluster.getEtcdProcessHostMap(readyHosts)

		if err := services.ReloadEtcdCluster(ctx, readyHosts, currentCluster.LocalConnDialerFactory, clientCert, clientkey, currentCluster.PrivateRegistriesMap, etcdProcessHostMap); err != nil {
			return err
		}
	}
	return nil
}

func syncLabels(ctx context.Context, currentCluster, kubeCluster *Cluster) {
	currentHosts := hosts.GetUniqueHostList(currentCluster.EtcdHosts, currentCluster.ControlPlaneHosts, currentCluster.WorkerHosts)
	configHosts := hosts.GetUniqueHostList(kubeCluster.EtcdHosts, kubeCluster.ControlPlaneHosts, kubeCluster.WorkerHosts)
	for _, host := range configHosts {
		for _, currentHost := range currentHosts {
			if host.Address == currentHost.Address {
				for k, v := range currentHost.Labels {
					if _, ok := host.Labels[k]; !ok {
						host.ToDelLabels[k] = v
					}
				}
				break
			}
		}
	}
}

func getReadyEtcdHosts(etcdHosts []*hosts.Host) []*hosts.Host {
	readyEtcdHosts := []*hosts.Host{}
	for _, host := range etcdHosts {
		if !host.ToAddEtcdMember {
			readyEtcdHosts = append(readyEtcdHosts, host)
			host.ExistingEtcdCluster = true
		}
	}
	return readyEtcdHosts
}
