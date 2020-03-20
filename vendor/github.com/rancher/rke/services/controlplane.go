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
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/drain"
)

func RunControlPlane(ctx context.Context, controlHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, cpNodePlanMap map[string]v3.RKEConfigNodePlan, updateWorkersOnly bool, alpineImage string, certMap map[string]pki.CertificatePKI) error {
	if updateWorkersOnly {
		return nil
	}
	log.Infof(ctx, "[%s] Building up Controller Plane..", ControlRole)
	var errgrp errgroup.Group

	hostsQueue := util.GetObjectQueue(controlHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				err := doDeployControlHost(ctx, runHost, localConnDialerFactory, prsMap, cpNodePlanMap[runHost.Address].Processes, alpineImage, certMap)
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
	log.Infof(ctx, "[%s] Successfully started Controller Plane..", ControlRole)
	return nil
}

func UpgradeControlPlaneNodes(ctx context.Context, kubeClient *kubernetes.Clientset, controlHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory,
	prsMap map[string]v3.PrivateRegistry, cpNodePlanMap map[string]v3.RKEConfigNodePlan, updateWorkersOnly bool, alpineImage string, certMap map[string]pki.CertificatePKI,
	upgradeStrategy *v3.NodeUpgradeStrategy, newHosts, inactiveHosts map[string]bool, maxUnavailable int) (string, error) {
	if updateWorkersOnly {
		return "", nil
	}
	var errMsgMaxUnavailableNotFailed string
	var drainHelper drain.Helper
	log.Infof(ctx, "[%s] Processing controlplane hosts for upgrade %v at a time", ControlRole, maxUnavailable)
	if len(newHosts) > 0 {
		var nodes []string
		for _, host := range controlHosts {
			if newHosts[host.HostnameOverride] {
				nodes = append(nodes, host.HostnameOverride)
			}
		}
		if len(nodes) > 0 {
			log.Infof(ctx, "[%s] Adding controlplane nodes %v to the cluster", ControlRole, strings.Join(nodes, ","))
		}
	}
	if upgradeStrategy.Drain {
		drainHelper = getDrainHelper(kubeClient, *upgradeStrategy)
		log.Infof(ctx, "[%s] Parameters provided to drain command: %#v", ControlRole, fmt.Sprintf("Force: %v, IgnoreAllDaemonSets: %v, DeleteLocalData: %v, Timeout: %v, GracePeriodSeconds: %v", drainHelper.Force, drainHelper.IgnoreAllDaemonSets, drainHelper.DeleteLocalData, drainHelper.Timeout, drainHelper.GracePeriodSeconds))
	}
	var inactiveHostErr error
	if len(inactiveHosts) > 0 {
		var inactiveHostNames []string
		for hostName := range inactiveHosts {
			inactiveHostNames = append(inactiveHostNames, hostName)
		}
		inactiveHostErr = fmt.Errorf("provisioning incomplete, host(s) [%s] skipped because they could not be contacted", strings.Join(inactiveHostNames, ","))
	}
	hostsFailedToUpgrade, err := processControlPlaneForUpgrade(ctx, kubeClient, controlHosts, localConnDialerFactory, prsMap, cpNodePlanMap, updateWorkersOnly, alpineImage, certMap,
		upgradeStrategy, newHosts, inactiveHosts, maxUnavailable, drainHelper)
	if err != nil || inactiveHostErr != nil {
		if len(hostsFailedToUpgrade) > 0 {
			logrus.Errorf("Failed to upgrade hosts: %v with error %v", strings.Join(hostsFailedToUpgrade, ","), err)
			errMsgMaxUnavailableNotFailed = fmt.Sprintf("Failed to upgrade hosts: %v with error %v", strings.Join(hostsFailedToUpgrade, ","), err)
		}
		var errors []error
		for _, e := range []error{err, inactiveHostErr} {
			if e != nil {
				errors = append(errors, e)
			}
		}
		return errMsgMaxUnavailableNotFailed, util.ErrList(errors)
	}
	log.Infof(ctx, "[%s] Successfully upgraded Controller Plane..", ControlRole)
	return errMsgMaxUnavailableNotFailed, nil
}

func processControlPlaneForUpgrade(ctx context.Context, kubeClient *kubernetes.Clientset, controlHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory,
	prsMap map[string]v3.PrivateRegistry, cpNodePlanMap map[string]v3.RKEConfigNodePlan, updateWorkersOnly bool, alpineImage string, certMap map[string]pki.CertificatePKI,
	upgradeStrategy *v3.NodeUpgradeStrategy, newHosts, inactiveHosts map[string]bool, maxUnavailable int, drainHelper drain.Helper) ([]string, error) {
	var errgrp errgroup.Group
	var failedHosts []string
	var hostsFailedToUpgrade = make(chan string, maxUnavailable)
	var hostsFailed sync.Map

	currentHostsPool := make(map[string]bool)
	for _, host := range controlHosts {
		currentHostsPool[host.HostnameOverride] = true
	}
	// upgrade control plane hosts maxUnavailable nodes time for zero downtime upgrades
	hostsQueue := util.GetObjectQueue(controlHosts)
	for w := 0; w < maxUnavailable; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				log.Infof(ctx, "Processing controlplane host %v", runHost.HostnameOverride)
				if newHosts[runHost.HostnameOverride] {
					if err := startNewControlHost(ctx, runHost, localConnDialerFactory, prsMap, cpNodePlanMap, updateWorkersOnly, alpineImage, certMap); err != nil {
						errList = append(errList, err)
						hostsFailedToUpgrade <- runHost.HostnameOverride
						hostsFailed.Store(runHost.HostnameOverride, true)
						break
					}
					continue
				}
				if err := CheckNodeReady(kubeClient, runHost, ControlRole); err != nil {
					errList = append(errList, err)
					hostsFailedToUpgrade <- runHost.HostnameOverride
					hostsFailed.Store(runHost.HostnameOverride, true)
					break
				}
				nodes, err := getNodeListForUpgrade(kubeClient, &sync.Map{}, newHosts, inactiveHosts, ControlRole)
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
				controlPlaneUpgradable, workerPlaneUpgradable, err := checkHostUpgradable(ctx, runHost, cpNodePlanMap)
				if err != nil {
					errList = append(errList, err)
					hostsFailedToUpgrade <- runHost.HostnameOverride
					hostsFailed.Store(runHost.HostnameOverride, true)
					break
				}
				if !controlPlaneUpgradable && !workerPlaneUpgradable {
					log.Infof(ctx, "Upgrade not required for controlplane and worker components of host %v", runHost.HostnameOverride)
					if err := k8s.CordonUncordon(kubeClient, runHost.HostnameOverride, false); err != nil {
						// This node didn't undergo an upgrade, so RKE will only log any error after uncordoning it and won't count this in maxUnavailable
						logrus.Errorf("[controlplane] Failed to uncordon node %v, error: %v", runHost.HostnameOverride, err)
					}
					continue
				}
				if err := upgradeControlHost(ctx, kubeClient, runHost, upgradeStrategy.Drain, drainHelper, localConnDialerFactory, prsMap, cpNodePlanMap, updateWorkersOnly, alpineImage, certMap, controlPlaneUpgradable, workerPlaneUpgradable); err != nil {
					errList = append(errList, err)
					hostsFailedToUpgrade <- runHost.HostnameOverride
					hostsFailed.Store(runHost.HostnameOverride, true)
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

func startNewControlHost(ctx context.Context, runHost *hosts.Host, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry,
	cpNodePlanMap map[string]v3.RKEConfigNodePlan, updateWorkersOnly bool, alpineImage string, certMap map[string]pki.CertificatePKI) error {
	if err := doDeployControlHost(ctx, runHost, localConnDialerFactory, prsMap, cpNodePlanMap[runHost.Address].Processes, alpineImage, certMap); err != nil {
		return err
	}
	if err := doDeployWorkerPlaneHost(ctx, runHost, localConnDialerFactory, prsMap, cpNodePlanMap[runHost.Address].Processes, certMap, updateWorkersOnly, alpineImage); err != nil {
		return err
	}
	return nil
}

func checkHostUpgradable(ctx context.Context, runHost *hosts.Host, cpNodePlanMap map[string]v3.RKEConfigNodePlan) (bool, bool, error) {
	var controlPlaneUpgradable, workerPlaneUpgradable bool
	controlPlaneUpgradable, err := isControlPlaneHostUpgradable(ctx, runHost, cpNodePlanMap[runHost.Address].Processes)
	if err != nil {
		return controlPlaneUpgradable, workerPlaneUpgradable, err
	}
	workerPlaneUpgradable, err = isWorkerHostUpgradable(ctx, runHost, cpNodePlanMap[runHost.Address].Processes)
	if err != nil {
		return controlPlaneUpgradable, workerPlaneUpgradable, err
	}
	return controlPlaneUpgradable, workerPlaneUpgradable, nil
}

func upgradeControlHost(ctx context.Context, kubeClient *kubernetes.Clientset, host *hosts.Host, drain bool, drainHelper drain.Helper,
	localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, cpNodePlanMap map[string]v3.RKEConfigNodePlan, updateWorkersOnly bool,
	alpineImage string, certMap map[string]pki.CertificatePKI, controlPlaneUpgradable, workerPlaneUpgradable bool) error {
	if err := cordonAndDrainNode(kubeClient, host, drain, drainHelper, ControlRole); err != nil {
		return err
	}
	if controlPlaneUpgradable {
		log.Infof(ctx, "Upgrading controlplane components for control host %v", host.HostnameOverride)
		if err := doDeployControlHost(ctx, host, localConnDialerFactory, prsMap, cpNodePlanMap[host.Address].Processes, alpineImage, certMap); err != nil {
			return err
		}
	}
	if workerPlaneUpgradable {
		log.Infof(ctx, "Upgrading workerplane components for control host %v", host.HostnameOverride)
		if err := doDeployWorkerPlaneHost(ctx, host, localConnDialerFactory, prsMap, cpNodePlanMap[host.Address].Processes, certMap, updateWorkersOnly, alpineImage); err != nil {
			return err
		}
	}

	if err := CheckNodeReady(kubeClient, host, ControlRole); err != nil {
		return err
	}
	if err := k8s.CordonUncordon(kubeClient, host.HostnameOverride, false); err != nil {
		return err
	}
	return nil
}

func RemoveControlPlane(ctx context.Context, controlHosts []*hosts.Host, force bool) error {
	log.Infof(ctx, "[%s] Tearing down the Controller Plane..", ControlRole)
	var errgrp errgroup.Group
	hostsQueue := util.GetObjectQueue(controlHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				if err := removeKubeAPI(ctx, runHost); err != nil {
					errList = append(errList, err)
				}
				if err := removeKubeController(ctx, runHost); err != nil {
					errList = append(errList, err)
				}
				if err := removeScheduler(ctx, runHost); err != nil {
					errList = append(errList, err)
				}
				// force is true in remove, false in reconcile
				if !runHost.IsWorker || !runHost.IsEtcd || force {
					if err := removeKubelet(ctx, runHost); err != nil {
						errList = append(errList, err)
					}
					if err := removeKubeproxy(ctx, runHost); err != nil {
						errList = append(errList, err)
					}
					if err := removeSidekick(ctx, runHost); err != nil {
						errList = append(errList, err)
					}
				}
			}
			return util.ErrList(errList)
		})
	}

	if err := errgrp.Wait(); err != nil {
		return err
	}

	log.Infof(ctx, "[%s] Successfully tore down Controller Plane..", ControlRole)
	return nil
}

func RestartControlPlane(ctx context.Context, controlHosts []*hosts.Host) error {
	log.Infof(ctx, "[%s] Restarting the Controller Plane..", ControlRole)
	var errgrp errgroup.Group

	hostsQueue := util.GetObjectQueue(controlHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				// restart KubeAPI
				if err := RestartKubeAPI(ctx, runHost); err != nil {
					errList = append(errList, err)
				}

				// restart KubeController
				if err := RestartKubeController(ctx, runHost); err != nil {
					errList = append(errList, err)
				}

				// restart scheduler
				err := RestartScheduler(ctx, runHost)
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
	log.Infof(ctx, "[%s] Successfully restarted Controller Plane..", ControlRole)
	return nil
}

func doDeployControlHost(ctx context.Context, host *hosts.Host, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, processMap map[string]v3.Process, alpineImage string, certMap map[string]pki.CertificatePKI) error {
	if host.IsWorker {
		if err := removeNginxProxy(ctx, host); err != nil {
			return err
		}
	}
	// run sidekick
	if err := runSidekick(ctx, host, prsMap, processMap[SidekickContainerName]); err != nil {
		return err
	}
	// run kubeapi
	if err := runKubeAPI(ctx, host, localConnDialerFactory, prsMap, processMap[KubeAPIContainerName], alpineImage, certMap); err != nil {
		return err
	}
	// run kubecontroller
	if err := runKubeController(ctx, host, localConnDialerFactory, prsMap, processMap[KubeControllerContainerName], alpineImage); err != nil {
		return err
	}
	// run scheduler
	return runScheduler(ctx, host, localConnDialerFactory, prsMap, processMap[SchedulerContainerName], alpineImage)
}

func isControlPlaneHostUpgradable(ctx context.Context, host *hosts.Host, processMap map[string]v3.Process) (bool, error) {
	for _, service := range []string{SidekickContainerName, KubeAPIContainerName, KubeControllerContainerName, SchedulerContainerName} {
		process := processMap[service]
		imageCfg, hostCfg, _ := GetProcessConfig(process, host)
		upgradable, err := docker.IsContainerUpgradable(ctx, host.DClient, imageCfg, hostCfg, service, host.Address, ControlRole)
		if err != nil {
			if client.IsErrNotFound(err) {
				// doDeployControlHost should be called so this container gets recreated
				logrus.Debugf("[%s] Host %v is upgradable because %v needs to run", ControlRole, host.HostnameOverride, service)
				return true, nil
			}
			return false, err
		}
		if upgradable {
			logrus.Debugf("[%s] Host %v is upgradable because %v has changed", ControlRole, host.HostnameOverride, service)
			// host upgradable even if a single service is upgradable
			return true, nil
		}
	}
	logrus.Debugf("[%s] Host %v is not upgradable", ControlRole, host.HostnameOverride)
	return false, nil
}
