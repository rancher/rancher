package services

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/docker/docker/client"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/util"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
)

const (
	unschedulableEtcdTaint    = "node-role.kubernetes.io/etcd=true:NoExecute"
	unschedulableControlTaint = "node-role.kubernetes.io/controlplane=true:NoSchedule"
)

func RunWorkerPlane(ctx context.Context, allHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, workerNodePlanMap map[string]v3.RKEConfigNodePlan, certMap map[string]pki.CertificatePKI, updateWorkersOnly bool, alpineImage string) error {
	log.Infof(ctx, "[%s] Building up Worker Plane..", WorkerRole)
	var errgrp errgroup.Group

	hostsQueue := util.GetObjectQueue(allHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				err := doDeployWorkerPlaneHost(ctx, runHost, localConnDialerFactory, prsMap, workerNodePlanMap[runHost.Address].Processes, certMap, updateWorkersOnly, alpineImage)
				if err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}

	if err := errgrp.Wait(); err != nil {
		return err
	}
	log.Infof(ctx, "[%s] Successfully started Worker Plane..", WorkerRole)
	return nil
}

func UpgradeWorkerPlaneForWorkerAndEtcdNodes(ctx context.Context, kubeClient *kubernetes.Clientset, mixedRolesHosts []*hosts.Host, workerOnlyHosts []*hosts.Host, inactiveHosts map[string]bool, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, workerNodePlanMap map[string]v3.RKEConfigNodePlan, certMap map[string]pki.CertificatePKI, updateWorkersOnly bool, alpineImage string, upgradeStrategy *v3.NodeUpgradeStrategy, newHosts map[string]bool, maxUnavailable int) (string, error) {
	log.Infof(ctx, "[%s] Upgrading Worker Plane..", WorkerRole)
	var errMsgMaxUnavailableNotFailed string
	updateNewHostsList(kubeClient, append(mixedRolesHosts, workerOnlyHosts...), newHosts)
	if len(mixedRolesHosts) > 0 {
		log.Infof(ctx, "First checking and processing worker components for upgrades on nodes with etcd role one at a time")
	}
	multipleRolesHostsFailedToUpgrade, err := processWorkerPlaneForUpgrade(ctx, kubeClient, mixedRolesHosts, localConnDialerFactory, prsMap, workerNodePlanMap, certMap, updateWorkersOnly, alpineImage, 1, upgradeStrategy, newHosts, inactiveHosts)
	if err != nil {
		logrus.Errorf("Failed to upgrade hosts: %v with error %v", strings.Join(multipleRolesHostsFailedToUpgrade, ","), err)
		return errMsgMaxUnavailableNotFailed, err
	}

	if len(workerOnlyHosts) > 0 {
		log.Infof(ctx, "Now checking and upgrading worker components on nodes with only worker role %v at a time", maxUnavailable)
	}
	workerOnlyHostsFailedToUpgrade, err := processWorkerPlaneForUpgrade(ctx, kubeClient, workerOnlyHosts, localConnDialerFactory, prsMap, workerNodePlanMap, certMap, updateWorkersOnly, alpineImage, maxUnavailable, upgradeStrategy, newHosts, inactiveHosts)
	if err != nil {
		logrus.Errorf("Failed to upgrade hosts: %v with error %v", strings.Join(workerOnlyHostsFailedToUpgrade, ","), err)
		if len(workerOnlyHostsFailedToUpgrade) >= maxUnavailable {
			return errMsgMaxUnavailableNotFailed, err
		}
		errMsgMaxUnavailableNotFailed = fmt.Sprintf("Failed to upgrade hosts: %v with error %v", strings.Join(workerOnlyHostsFailedToUpgrade, ","), err)
	}

	log.Infof(ctx, "[%s] Successfully upgraded Worker Plane..", WorkerRole)
	return errMsgMaxUnavailableNotFailed, nil
}

func updateNewHostsList(kubeClient *kubernetes.Clientset, allHosts []*hosts.Host, newHosts map[string]bool) {
	for _, h := range allHosts {
		_, err := k8s.GetNode(kubeClient, h.HostnameOverride)
		if err != nil && apierrors.IsNotFound(err) {
			// this host could have been added to cluster state upon successful controlplane upgrade but isn't a node yet.
			newHosts[h.HostnameOverride] = true
		}
	}
}

func processWorkerPlaneForUpgrade(ctx context.Context, kubeClient *kubernetes.Clientset, allHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory,
	prsMap map[string]v3.PrivateRegistry, workerNodePlanMap map[string]v3.RKEConfigNodePlan, certMap map[string]pki.CertificatePKI, updateWorkersOnly bool, alpineImage string,
	maxUnavailable int, upgradeStrategy *v3.NodeUpgradeStrategy, newHosts, inactiveHosts map[string]bool) ([]string, error) {
	var errgrp errgroup.Group
	var drainHelper drain.Helper
	var failedHosts []string
	var hostsFailedToUpgrade = make(chan string, maxUnavailable)
	var hostsFailed sync.Map

	hostsQueue := util.GetObjectQueue(allHosts)
	if upgradeStrategy.Drain {
		drainHelper = getDrainHelper(kubeClient, *upgradeStrategy)
		log.Infof(ctx, "[%s] Parameters provided to drain command: %#v", WorkerRole, fmt.Sprintf("Force: %v, IgnoreAllDaemonSets: %v, DeleteLocalData: %v, Timeout: %v, GracePeriodSeconds: %v", drainHelper.Force, drainHelper.IgnoreAllDaemonSets, drainHelper.DeleteLocalData, drainHelper.Timeout, drainHelper.GracePeriodSeconds))

	}
	currentHostsPool := make(map[string]bool)
	for _, host := range allHosts {
		currentHostsPool[host.HostnameOverride] = true
	}
	/* Each worker thread starts a goroutine that reads the hostsQueue channel in a for loop
	Using same number of worker threads as maxUnavailable ensures only maxUnavailable number of nodes are being processed at a time
	Node is done upgrading only after it is listed as ready and uncordoned.*/
	for w := 0; w < maxUnavailable; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				logrus.Infof("[workerplane] Processing host %v", runHost.HostnameOverride)
				if newHosts[runHost.HostnameOverride] {
					if err := doDeployWorkerPlaneHost(ctx, runHost, localConnDialerFactory, prsMap, workerNodePlanMap[runHost.Address].Processes, certMap, updateWorkersOnly, alpineImage); err != nil {
						errList = append(errList, err)
						hostsFailedToUpgrade <- runHost.HostnameOverride
						hostsFailed.Store(runHost.HostnameOverride, true)
						break
					}
					continue
				}
				if err := CheckNodeReady(kubeClient, runHost, WorkerRole); err != nil {
					errList = append(errList, err)
					hostsFailed.Store(runHost.HostnameOverride, true)
					hostsFailedToUpgrade <- runHost.HostnameOverride
					break
				}
				nodes, err := getNodeListForUpgrade(kubeClient, &hostsFailed, newHosts, inactiveHosts, WorkerRole)
				if err != nil {
					errList = append(errList, err)
				}
				var maxUnavailableHit bool
				for _, node := range nodes {
					// in case any previously added nodes or till now unprocessed nodes become unreachable during upgrade
					if !k8s.IsNodeReady(node) && currentHostsPool[node.Labels[k8s.HostnameLabel]] {
						if len(hostsFailedToUpgrade) >= maxUnavailable {
							maxUnavailableHit = true
							break
						}
						hostsFailed.Store(node.Labels[k8s.HostnameLabel], true)
						hostsFailedToUpgrade <- node.Labels[k8s.HostnameLabel]
						errList = append(errList, fmt.Errorf("host %v not ready", node.Labels[k8s.HostnameLabel]))
					}
				}
				if maxUnavailableHit || len(hostsFailedToUpgrade) >= maxUnavailable {
					break
				}
				upgradable, err := isWorkerHostUpgradable(ctx, runHost, workerNodePlanMap[runHost.Address].Processes)
				if err != nil {
					errList = append(errList, err)
					hostsFailed.Store(runHost.HostnameOverride, true)
					hostsFailedToUpgrade <- runHost.HostnameOverride
					break
				}
				if !upgradable {
					logrus.Infof("[workerplane] Upgrade not required for worker components of host %v", runHost.HostnameOverride)
					if err := k8s.CordonUncordon(kubeClient, runHost.HostnameOverride, false); err != nil {
						// This node didn't undergo an upgrade, so RKE will only log any error after uncordoning it and won't count this in maxUnavailable
						logrus.Errorf("[workerplane] Failed to uncordon node %v, error: %v", runHost.HostnameOverride, err)
					}
					continue
				}
				if err := upgradeWorkerHost(ctx, kubeClient, runHost, upgradeStrategy.Drain, drainHelper, localConnDialerFactory, prsMap, workerNodePlanMap, certMap, updateWorkersOnly, alpineImage); err != nil {
					errList = append(errList, err)
					hostsFailed.Store(runHost.HostnameOverride, true)
					hostsFailedToUpgrade <- runHost.HostnameOverride
					break
				}
			}
			return util.ErrList(errList)
		})
	}

	err := errgrp.Wait()
	close(hostsFailedToUpgrade)
	if err != nil {
		for host := range hostsFailedToUpgrade {
			failedHosts = append(failedHosts, host)
		}
	}
	return failedHosts, err
}

func upgradeWorkerHost(ctx context.Context, kubeClient *kubernetes.Clientset, runHost *hosts.Host, drainFlag bool, drainHelper drain.Helper,
	localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, workerNodePlanMap map[string]v3.RKEConfigNodePlan, certMap map[string]pki.CertificatePKI, updateWorkersOnly bool,
	alpineImage string) error {
	// cordon and drain
	if err := cordonAndDrainNode(kubeClient, runHost, drainFlag, drainHelper, WorkerRole); err != nil {
		return err
	}
	logrus.Debugf("[workerplane] upgrading host %v", runHost.HostnameOverride)
	if err := doDeployWorkerPlaneHost(ctx, runHost, localConnDialerFactory, prsMap, workerNodePlanMap[runHost.Address].Processes, certMap, updateWorkersOnly, alpineImage); err != nil {
		return err
	}
	// consider upgrade done when kubeclient lists node as ready
	if err := CheckNodeReady(kubeClient, runHost, WorkerRole); err != nil {
		return err
	}
	// uncordon node
	if err := k8s.CordonUncordon(kubeClient, runHost.HostnameOverride, false); err != nil {
		return err
	}
	return nil
}

func doDeployWorkerPlaneHost(ctx context.Context, host *hosts.Host, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, processMap map[string]v3.Process, certMap map[string]pki.CertificatePKI, updateWorkersOnly bool, alpineImage string) error {
	if updateWorkersOnly {
		if !host.UpdateWorker {
			return nil
		}
	}
	if !host.IsWorker {
		if host.IsEtcd {
			// Add unschedulable taint
			host.ToAddTaints = append(host.ToAddTaints, unschedulableEtcdTaint)
		}
		if host.IsControl {
			// Add unschedulable taint
			host.ToAddTaints = append(host.ToAddTaints, unschedulableControlTaint)
		}
	}
	return doDeployWorkerPlane(ctx, host, localConnDialerFactory, prsMap, processMap, certMap, alpineImage)
}

func RemoveWorkerPlane(ctx context.Context, workerHosts []*hosts.Host, force bool) error {
	log.Infof(ctx, "[%s] Tearing down Worker Plane..", WorkerRole)
	var errgrp errgroup.Group
	hostsQueue := util.GetObjectQueue(workerHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				if runHost.IsControl && !force {
					log.Infof(ctx, "[%s] Host [%s] is already a controlplane host, nothing to do.", WorkerRole, runHost.Address)
					return nil
				}
				if err := removeKubelet(ctx, runHost); err != nil {
					errList = append(errList, err)
				}
				if err := removeKubeproxy(ctx, runHost); err != nil {
					errList = append(errList, err)
				}
				if err := removeNginxProxy(ctx, runHost); err != nil {
					errList = append(errList, err)
				}
				if err := removeSidekick(ctx, runHost); err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}

	if err := errgrp.Wait(); err != nil {
		return err
	}
	log.Infof(ctx, "[%s] Successfully tore down Worker Plane..", WorkerRole)

	return nil
}

func RestartWorkerPlane(ctx context.Context, workerHosts []*hosts.Host) error {
	log.Infof(ctx, "[%s] Restarting Worker Plane..", WorkerRole)
	var errgrp errgroup.Group

	hostsQueue := util.GetObjectQueue(workerHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				if err := RestartKubelet(ctx, runHost); err != nil {
					errList = append(errList, err)
				}
				if err := RestartKubeproxy(ctx, runHost); err != nil {
					errList = append(errList, err)
				}
				if err := RestartNginxProxy(ctx, runHost); err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	log.Infof(ctx, "[%s] Successfully restarted Worker Plane..", WorkerRole)

	return nil
}

func doDeployWorkerPlane(ctx context.Context, host *hosts.Host,
	localConnDialerFactory hosts.DialerFactory,
	prsMap map[string]v3.PrivateRegistry, processMap map[string]v3.Process, certMap map[string]pki.CertificatePKI, alpineImage string) error {
	// run nginx proxy
	if !host.IsControl {
		if err := runNginxProxy(ctx, host, prsMap, processMap[NginxProxyContainerName], alpineImage); err != nil {
			return err
		}
	}
	// run sidekick
	if err := runSidekick(ctx, host, prsMap, processMap[SidekickContainerName]); err != nil {
		return err
	}
	// run kubelet
	if err := runKubelet(ctx, host, localConnDialerFactory, prsMap, processMap[KubeletContainerName], certMap, alpineImage); err != nil {
		return err
	}
	return runKubeproxy(ctx, host, localConnDialerFactory, prsMap, processMap[KubeproxyContainerName], alpineImage)
}

func isWorkerHostUpgradable(ctx context.Context, host *hosts.Host, processMap map[string]v3.Process) (bool, error) {
	for _, service := range []string{NginxProxyContainerName, SidekickContainerName, KubeletContainerName, KubeproxyContainerName} {
		process := processMap[service]
		imageCfg, hostCfg, _ := GetProcessConfig(process, host)
		upgradable, err := docker.IsContainerUpgradable(ctx, host.DClient, imageCfg, hostCfg, service, host.Address, WorkerRole)
		if err != nil {
			if client.IsErrNotFound(err) {
				if service == NginxProxyContainerName && host.IsControl {
					// nginxProxy should not exist on control hosts, so no changes needed
					continue
				}
				// doDeployWorkerPlane should be called so this container gets recreated
				logrus.Debugf("[%s] Host %v is upgradable because %v needs to run", WorkerRole, host.HostnameOverride, service)
				return true, nil
			}
			return false, err
		}
		if upgradable {
			logrus.Debugf("[%s] Host %v is upgradable because %v has changed", WorkerRole, host.HostnameOverride, service)
			// host upgradable even if a single service is upgradable
			return true, nil
		}
	}
	logrus.Debugf("[%s] Host %v is not upgradable", WorkerRole, host.HostnameOverride)
	return false, nil
}
