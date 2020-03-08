package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/pki/cert"
	"github.com/rancher/rke/services"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

const (
	unschedulableEtcdTaint    = "node-role.kubernetes.io/etcd=true:NoExecute"
	unschedulableControlTaint = "node-role.kubernetes.io/controlplane=true:NoSchedule"

	unschedulableEtcdTaintKey    = "node-role.kubernetes.io/etcd=:NoExecute"
	unschedulableControlTaintKey = "node-role.kubernetes.io/controlplane=:NoSchedule"

	EtcdPlaneNodesReplacedErr = "Etcd plane nodes are replaced. Stopping provisioning. Please restore your cluster from backup."
)

func ReconcileCluster(ctx context.Context, kubeCluster, currentCluster *Cluster, flags ExternalFlags, svcOptionData map[string]*v3.KubernetesServicesOptions) error {
	logrus.Tracef("[reconcile] currentCluster: %+v\n", currentCluster)
	log.Infof(ctx, "[reconcile] Reconciling cluster state")
	kubeCluster.UpdateWorkersOnly = flags.UpdateOnly
	if currentCluster == nil {
		log.Infof(ctx, "[reconcile] This is newly generated cluster")
		kubeCluster.UpdateWorkersOnly = false
		return nil
	}
	// If certificates are not present, this is broken state and should error out
	if len(currentCluster.Certificates) == 0 {
		return fmt.Errorf("Certificates are not present in cluster state, recover rkestate file or certificate information in cluster state")
	}

	kubeClient, err := k8s.NewClient(kubeCluster.LocalKubeConfigPath, kubeCluster.K8sWrapTransport)
	if err != nil {
		return fmt.Errorf("Failed to initialize new kubernetes client: %v", err)
	}
	// sync node labels to define the toDelete labels
	syncLabels(ctx, currentCluster, kubeCluster)
	syncNodeRoles(ctx, currentCluster, kubeCluster)

	if err := reconcileEtcd(ctx, currentCluster, kubeCluster, kubeClient, svcOptionData); err != nil {
		return fmt.Errorf("Failed to reconcile etcd plane: %v", err)
	}

	if err := reconcileWorker(ctx, currentCluster, kubeCluster, kubeClient); err != nil {
		return err
	}

	if err := reconcileControl(ctx, currentCluster, kubeCluster, kubeClient); err != nil {
		return err
	}

	if kubeCluster.ForceDeployCerts {
		if err := restartComponentsWhenCertChanges(ctx, currentCluster, kubeCluster); err != nil {
			return err
		}
	}
	log.Infof(ctx, "[reconcile] Reconciled cluster state successfully")
	return nil
}

func reconcileWorker(ctx context.Context, currentCluster, kubeCluster *Cluster, kubeClient *kubernetes.Clientset) error {
	// worker deleted first to avoid issues when worker+controller on same host
	logrus.Debugf("[reconcile] Check worker hosts to be deleted")
	wpToDelete := hosts.GetToDeleteHosts(currentCluster.WorkerHosts, kubeCluster.WorkerHosts, kubeCluster.InactiveHosts, false)
	for _, toDeleteHost := range wpToDelete {
		toDeleteHost.IsWorker = false
		if err := hosts.DeleteNode(ctx, toDeleteHost, kubeClient, toDeleteHost.IsControl || toDeleteHost.IsEtcd, kubeCluster.CloudProvider.Name); err != nil {
			return fmt.Errorf("Failed to delete worker node [%s] from cluster: %v", toDeleteHost.Address, err)
		}
		// attempting to clean services/files on the host
		if err := reconcileHost(ctx, toDeleteHost, true, false, currentCluster.SystemImages.Alpine, currentCluster.DockerDialerFactory, currentCluster.PrivateRegistriesMap, currentCluster.PrefixPath, currentCluster.Version); err != nil {
			log.Warnf(ctx, "[reconcile] Couldn't clean up worker node [%s]: %v", toDeleteHost.Address, err)
			continue
		}
	}
	// attempt to remove unschedulable taint
	toAddHosts := hosts.GetToAddHosts(currentCluster.WorkerHosts, kubeCluster.WorkerHosts)
	for _, host := range toAddHosts {
		host.UpdateWorker = true
		if host.IsEtcd {
			host.ToDelTaints = append(host.ToDelTaints, unschedulableEtcdTaint)
		}
		if host.IsControl {
			host.ToDelTaints = append(host.ToDelTaints, unschedulableControlTaint)
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
	cpToDelete := hosts.GetToDeleteHosts(currentCluster.ControlPlaneHosts, kubeCluster.ControlPlaneHosts, kubeCluster.InactiveHosts, false)
	// move the current host in local kubeconfig to the end of the list
	for i, toDeleteHost := range cpToDelete {
		if toDeleteHost.Address == selfDeleteAddress {
			cpToDelete = append(cpToDelete[:i], cpToDelete[i+1:]...)
			cpToDelete = append(cpToDelete, toDeleteHost)
		}
	}
	if len(cpToDelete) == len(currentCluster.ControlPlaneHosts) {
		log.Infof(ctx, "[reconcile] Deleting all current controlplane nodes, skipping deleting from k8s cluster")
		// rebuilding local admin config to enable saving cluster state
		if err := rebuildLocalAdminConfig(ctx, kubeCluster); err != nil {
			return err
		}
		return nil
	}
	for _, toDeleteHost := range cpToDelete {
		if err := cleanControlNode(ctx, kubeCluster, currentCluster, toDeleteHost); err != nil {
			return err
		}
	}
	// rebuilding local admin config to enable saving cluster state
	if err := rebuildLocalAdminConfig(ctx, kubeCluster); err != nil {
		return err
	}
	return nil
}

func reconcileHost(ctx context.Context, toDeleteHost *hosts.Host, worker, etcd bool, cleanerImage string, dialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, clusterPrefixPath string, clusterVersion string) error {
	var retryErr error
	retries := 3
	sleepSeconds := 3
	for i := 0; i < retries; i++ {
		if retryErr = toDeleteHost.TunnelUp(ctx, dialerFactory, clusterPrefixPath, clusterVersion); retryErr != nil {
			logrus.Debugf("Failed to dial the host %s trying again in %d seconds", toDeleteHost.Address, sleepSeconds)
			time.Sleep(time.Second * time.Duration(sleepSeconds))
			toDeleteHost.DClient = nil
			continue
		}
		break
	}
	if retryErr != nil {
		return fmt.Errorf("Not able to reach the host: %v", retryErr)
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

func reconcileEtcd(ctx context.Context, currentCluster, kubeCluster *Cluster, kubeClient *kubernetes.Clientset, svcOptionData map[string]*v3.KubernetesServicesOptions) error {
	etcdToDelete := hosts.GetToDeleteHosts(currentCluster.EtcdHosts, kubeCluster.EtcdHosts, kubeCluster.InactiveHosts, false)
	etcdToAdd := hosts.GetToAddHosts(currentCluster.EtcdHosts, kubeCluster.EtcdHosts)
	clientCert := cert.EncodeCertPEM(currentCluster.Certificates[pki.KubeNodeCertName].Certificate)
	clientKey := cert.EncodePrivateKeyPEM(currentCluster.Certificates[pki.KubeNodeCertName].Key)

	// check if the whole etcd plane is replaced
	if isEtcdPlaneReplaced(ctx, currentCluster, kubeCluster) {
		return fmt.Errorf("%v", EtcdPlaneNodesReplacedErr)
	}
	// check if Node changed its public IP
	for i := range etcdToDelete {
		for j := range etcdToAdd {
			if etcdToDelete[i].InternalAddress == etcdToAdd[j].InternalAddress {
				etcdToDelete[i].Address = etcdToAdd[j].Address
			}
			break
		}
	}
	// handle etcd member delete
	if err := deleteEtcdMembers(ctx, currentCluster, kubeCluster, kubeClient, clientCert, clientKey, etcdToDelete); err != nil {
		return err
	}
	// handle etcd member add
	return addEtcdMembers(ctx, currentCluster, kubeCluster, kubeClient, svcOptionData, clientCert, clientKey, etcdToAdd)
}

func addEtcdMembers(ctx context.Context, currentCluster, kubeCluster *Cluster, kubeClient *kubernetes.Clientset, svcOptionData map[string]*v3.KubernetesServicesOptions, clientCert, clientKey []byte, etcdToAdd []*hosts.Host) error {
	log.Infof(ctx, "[reconcile] Check etcd hosts to be added")
	for _, etcdHost := range etcdToAdd {
		kubeCluster.UpdateWorkersOnly = false
		etcdHost.ToAddEtcdMember = true
	}
	for _, etcdHost := range etcdToAdd {
		// Check if the host already part of the cluster -- this will cover cluster with lost quorum
		isEtcdMember, err := services.IsEtcdMember(ctx, etcdHost, kubeCluster.EtcdHosts, currentCluster.LocalConnDialerFactory, clientCert, clientKey)
		if err != nil {
			return err
		}
		if !isEtcdMember {
			if err := services.AddEtcdMember(ctx, etcdHost, kubeCluster.EtcdHosts, currentCluster.LocalConnDialerFactory, clientCert, clientKey); err != nil {
				return err
			}
		}
		etcdHost.ToAddEtcdMember = false
		kubeCluster.setReadyEtcdHosts()

		etcdNodePlanMap := make(map[string]v3.RKEConfigNodePlan)
		for _, etcdReadyHost := range kubeCluster.EtcdReadyHosts {
			svcOptions, err := kubeCluster.GetKubernetesServicesOptions(etcdReadyHost.DockerInfo.OSType, svcOptionData)
			if err != nil {
				return err
			}
			etcdNodePlanMap[etcdReadyHost.Address] = BuildRKEConfigNodePlan(ctx, kubeCluster, etcdReadyHost, etcdReadyHost.DockerInfo, svcOptions)
		}
		// this will start the newly added etcd node and make sure it started correctly before restarting other node
		// https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/runtime-configuration.md#add-a-new-member
		if err := services.ReloadEtcdCluster(ctx, kubeCluster.EtcdReadyHosts, etcdHost, currentCluster.LocalConnDialerFactory, clientCert, clientKey, currentCluster.PrivateRegistriesMap, etcdNodePlanMap, kubeCluster.SystemImages.Alpine); err != nil {
			return err
		}
	}
	return nil
}

func deleteEtcdMembers(ctx context.Context, currentCluster, kubeCluster *Cluster, kubeClient *kubernetes.Clientset, clientCert, clientKey []byte, etcdToDelete []*hosts.Host) error {
	log.Infof(ctx, "[reconcile] Check etcd hosts to be deleted")
	for _, etcdHost := range etcdToDelete {
		etcdHost.IsEtcd = false
		if err := services.RemoveEtcdMember(ctx, etcdHost, kubeCluster.EtcdHosts, currentCluster.LocalConnDialerFactory, clientCert, clientKey); err != nil {
			log.Warnf(ctx, "[reconcile] %v", err)
			continue
		}
		if err := hosts.DeleteNode(ctx, etcdHost, kubeClient, etcdHost.IsControl || etcdHost.IsWorker, kubeCluster.CloudProvider.Name); err != nil {
			log.Warnf(ctx, "Failed to delete etcd node [%s] from cluster: %v", etcdHost.Address, err)
			continue
		}
		// attempting to clean services/files on the host
		if err := reconcileHost(ctx, etcdHost, false, true, currentCluster.SystemImages.Alpine, currentCluster.DockerDialerFactory, currentCluster.PrivateRegistriesMap, currentCluster.PrefixPath, currentCluster.Version); err != nil {
			log.Warnf(ctx, "[reconcile] Couldn't clean up etcd node [%s]: %v", etcdHost.Address, err)
			continue
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

func syncNodeRoles(ctx context.Context, currentCluster, kubeCluster *Cluster) {
	currentHosts := hosts.GetUniqueHostList(currentCluster.EtcdHosts, currentCluster.ControlPlaneHosts, currentCluster.WorkerHosts)
	configHosts := hosts.GetUniqueHostList(kubeCluster.EtcdHosts, kubeCluster.ControlPlaneHosts, kubeCluster.WorkerHosts)
	for _, host := range configHosts {
		for _, currentHost := range currentHosts {
			if host.Address == currentHost.Address {
				currentHost.IsWorker = host.IsWorker
				currentHost.IsEtcd = host.IsEtcd
				currentHost.IsControl = host.IsControl
				break
			}
		}
	}
}

func (c *Cluster) setReadyEtcdHosts() {
	c.EtcdReadyHosts = []*hosts.Host{}
	for _, host := range c.EtcdHosts {
		if !host.ToAddEtcdMember {
			c.EtcdReadyHosts = append(c.EtcdReadyHosts, host)
			host.ExistingEtcdCluster = true
		}
	}
}

func cleanControlNode(ctx context.Context, kubeCluster, currentCluster *Cluster, toDeleteHost *hosts.Host) error {
	kubeClient, err := k8s.NewClient(kubeCluster.LocalKubeConfigPath, kubeCluster.K8sWrapTransport)
	if err != nil {
		return fmt.Errorf("Failed to initialize new kubernetes client: %v", err)
	}

	// if I am deleting a node that's already in the config, it's probably being replaced and I shouldn't remove it  from ks8
	if !hosts.IsNodeInList(toDeleteHost, kubeCluster.ControlPlaneHosts) {
		if err := hosts.DeleteNode(ctx, toDeleteHost, kubeClient, toDeleteHost.IsWorker || toDeleteHost.IsEtcd, kubeCluster.CloudProvider.Name); err != nil {
			return fmt.Errorf("Failed to delete controlplane node [%s] from cluster: %v", toDeleteHost.Address, err)
		}
	}
	// attempting to clean services/files on the host
	if err := reconcileHost(ctx, toDeleteHost, false, false, currentCluster.SystemImages.Alpine, currentCluster.DockerDialerFactory, currentCluster.PrivateRegistriesMap, currentCluster.PrefixPath, currentCluster.Version); err != nil {
		log.Warnf(ctx, "[reconcile] Couldn't clean up controlplane node [%s]: %v", toDeleteHost.Address, err)
	}
	return nil
}

func restartComponentsWhenCertChanges(ctx context.Context, currentCluster, kubeCluster *Cluster) error {
	AllCertsMap := map[string]bool{
		pki.KubeAPICertName:            false,
		pki.RequestHeaderCACertName:    false,
		pki.CACertName:                 false,
		pki.ServiceAccountTokenKeyName: false,
		pki.APIProxyClientCertName:     false,
		pki.KubeControllerCertName:     false,
		pki.KubeSchedulerCertName:      false,
		pki.KubeProxyCertName:          false,
		pki.KubeNodeCertName:           false,
	}
	checkCertificateChanges(ctx, currentCluster, kubeCluster, AllCertsMap)
	// check Restart Function
	allHosts := hosts.GetUniqueHostList(kubeCluster.EtcdHosts, kubeCluster.ControlPlaneHosts, kubeCluster.WorkerHosts)
	AllCertsFuncMap := map[string][]services.RestartFunc{
		pki.CACertName:                 []services.RestartFunc{services.RestartKubeAPI, services.RestartKubeController, services.RestartKubelet},
		pki.KubeAPICertName:            []services.RestartFunc{services.RestartKubeAPI, services.RestartKubeController},
		pki.RequestHeaderCACertName:    []services.RestartFunc{services.RestartKubeAPI},
		pki.ServiceAccountTokenKeyName: []services.RestartFunc{services.RestartKubeAPI, services.RestartKubeController},
		pki.APIProxyClientCertName:     []services.RestartFunc{services.RestartKubeAPI},
		pki.KubeControllerCertName:     []services.RestartFunc{services.RestartKubeController},
		pki.KubeSchedulerCertName:      []services.RestartFunc{services.RestartScheduler},
		pki.KubeProxyCertName:          []services.RestartFunc{services.RestartKubeproxy},
		pki.KubeNodeCertName:           []services.RestartFunc{services.RestartKubelet},
	}
	for certName, changed := range AllCertsMap {
		if changed {
			for _, host := range allHosts {
				runRestartFuncs(ctx, AllCertsFuncMap, certName, host)
			}
		}
	}

	for _, host := range kubeCluster.EtcdHosts {
		etcdCertName := pki.GetCrtNameForHost(host, pki.EtcdCertName)
		certMap := map[string]bool{
			etcdCertName: false,
		}
		checkCertificateChanges(ctx, currentCluster, kubeCluster, certMap)
		if certMap[etcdCertName] || AllCertsMap[pki.CACertName] {
			if err := docker.DoRestartContainer(ctx, host.DClient, services.EtcdContainerName, host.HostnameOverride); err != nil {
				return err
			}
		}
	}
	return nil
}

func runRestartFuncs(ctx context.Context, certFuncMap map[string][]services.RestartFunc, certName string, host *hosts.Host) error {
	for _, restartFunc := range certFuncMap[certName] {
		if err := restartFunc(ctx, host); err != nil {
			return err
		}
	}
	return nil
}

func checkCertificateChanges(ctx context.Context, currentCluster, kubeCluster *Cluster, certMap map[string]bool) {
	for certName := range certMap {
		if currentCluster.Certificates[certName].CertificatePEM != kubeCluster.Certificates[certName].CertificatePEM {
			certMap[certName] = true
			continue
		}
		if !(certName == pki.RequestHeaderCACertName || certName == pki.CACertName) {
			if currentCluster.Certificates[certName].KeyPEM != kubeCluster.Certificates[certName].KeyPEM {
				certMap[certName] = true
			}
		}
	}
}

func isEtcdPlaneReplaced(ctx context.Context, currentCluster, kubeCluster *Cluster) bool {
	numCurrentEtcdHosts := len(currentCluster.EtcdHosts)
	// We had and have no etcd hosts, nothing was replaced
	if numCurrentEtcdHosts == 0 && len(kubeCluster.EtcdHosts) == 0 {
		return false
	}

	// old etcd nodes are down, we added new ones
	numEtcdToDeleteInactive := len(hosts.GetToDeleteHosts(currentCluster.EtcdHosts, kubeCluster.EtcdHosts, kubeCluster.InactiveHosts, true))
	if numEtcdToDeleteInactive == numCurrentEtcdHosts {
		return true
	}

	// one or more etcd nodes are removed from cluster.yaml and replaced
	if len(hosts.GetHostListIntersect(kubeCluster.EtcdHosts, currentCluster.EtcdHosts)) == 0 {
		return true
	}
	return false
}

func syncTaints(ctx context.Context, currentCluster, kubeCluster *Cluster) {
	var currentHosts, expectedHosts []*hosts.Host
	var currentTaints, expectedTaints map[string]map[string]string
	// handling taints in configuration
	if currentCluster != nil {
		currentHosts = hosts.GetUniqueHostList(currentCluster.EtcdHosts, currentCluster.ControlPlaneHosts, currentCluster.WorkerHosts)
		currentTaints = getHostsTaintsMap(currentHosts)
	}
	expectedHosts = hosts.GetUniqueHostList(kubeCluster.EtcdHosts, kubeCluster.ControlPlaneHosts, kubeCluster.WorkerHosts)
	expectedTaints = getHostsTaintsMap(expectedHosts)

	for _, host := range expectedHosts {
		var toAddTaints, toDelTaints []string
		currentSet := currentTaints[host.Address]
		expectedSet := expectedTaints[host.Address]
		for key, expected := range expectedSet {
			current, ok := currentSet[key]
			// create or update taints in host.
			// by deleting the old taint and creating the new taint to do the update logic.
			if expected != current {
				toAddTaints = append(toAddTaints, expected)
				if ok { // if found but the values are different, the current taint will be deleted.
					toDelTaints = append(toDelTaints, current)
				}
			}
		}
		for key, current := range currentSet {
			_, ok := expectedSet[key]
			if !ok { // remove the taints which can't be found in the expected taints
				toDelTaints = append(toDelTaints, current)
			}
		}
		host.ToAddTaints = append(host.ToAddTaints, toAddTaints...)
		host.ToDelTaints = append(host.ToDelTaints, toDelTaints...)
	}
}

//getHostsTaintsMap return the taint set with unique key & effect for each host
func getHostsTaintsMap(list []*hosts.Host) map[string]map[string]string {
	rtn := make(map[string]map[string]string)
	for _, item := range list {
		set := make(map[string]string)
		for _, taint := range item.RKEConfigNode.Taints {
			key := getTaintKey(taint)
			value := getTaintValue(taint)
			if key == unschedulableEtcdTaintKey ||
				key == unschedulableControlTaintKey {
				logrus.Warnf("taint %s is reserved, ignore this taint", value)
				continue
			}
			if _, ok := set[key]; ok {
				logrus.Warnf("duplicated taint %s in host %s, ignore this taint", value, item.Address)
				continue
			}
			set[key] = value
		}
		if len(set) > 0 {
			rtn[item.Address] = set
		}
	}
	return rtn
}

func getTaintKey(taint v3.RKETaint) string {
	return fmt.Sprintf("%s=:%s", taint.Key, taint.Effect)
}

func getTaintValue(taint v3.RKETaint) string {
	return fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect)
}
