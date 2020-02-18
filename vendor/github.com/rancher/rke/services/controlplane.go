package services

import (
	"context"
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

func UpgradeControlPlane(ctx context.Context, kubeClient *kubernetes.Clientset, controlHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, cpNodePlanMap map[string]v3.RKEConfigNodePlan, updateWorkersOnly bool, alpineImage string, certMap map[string]pki.CertificatePKI, upgradeStrategy *v3.NodeUpgradeStrategy, newHosts map[string]bool) error {
	if updateWorkersOnly {
		return nil
	}
	var drainHelper drain.Helper

	log.Infof(ctx, "[%s] Processing control plane components for upgrade one at a time", ControlRole)
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
	}
	// upgrade control plane hosts one at a time for zero downtime upgrades
	for _, host := range controlHosts {
		log.Infof(ctx, "Processing controlplane host %v", host.HostnameOverride)
		if newHosts[host.HostnameOverride] {
			if err := doDeployControlHost(ctx, host, localConnDialerFactory, prsMap, cpNodePlanMap[host.Address].Processes, alpineImage, certMap); err != nil {
				return err
			}
			continue
		}
		nodes, err := getNodeListForUpgrade(kubeClient, &sync.Map{}, newHosts, false)
		if err != nil {
			return err
		}
		var maxUnavailableHit bool
		for _, node := range nodes {
			// in case any previously added nodes or till now unprocessed nodes become unreachable during upgrade
			if !k8s.IsNodeReady(node) {
				maxUnavailableHit = true
				break
			}
		}
		if maxUnavailableHit {
			return err
		}

		upgradable, err := isControlPlaneHostUpgradable(ctx, host, cpNodePlanMap[host.Address].Processes)
		if err != nil {
			return err
		}
		if !upgradable {
			log.Infof(ctx, "Upgrade not required for controlplane components of host %v", host.HostnameOverride)
			continue
		}
		if err := checkNodeReady(kubeClient, host, ControlRole); err != nil {
			return err
		}
		if err := cordonAndDrainNode(kubeClient, host, upgradeStrategy.Drain, drainHelper, ControlRole); err != nil {
			return err
		}
		if err := doDeployControlHost(ctx, host, localConnDialerFactory, prsMap, cpNodePlanMap[host.Address].Processes, alpineImage, certMap); err != nil {
			return err
		}
		if err := checkNodeReady(kubeClient, host, ControlRole); err != nil {
			return err
		}
		if err := k8s.CordonUncordon(kubeClient, host.HostnameOverride, false); err != nil {
			return err
		}
	}
	log.Infof(ctx, "[%s] Successfully upgraded Controller Plane..", ControlRole)
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
