package services

import (
	"context"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"golang.org/x/sync/errgroup"
)

func RunControlPlane(ctx context.Context, controlHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, cpNodePlanMap map[string]v3.RKEConfigNodePlan, updateWorkersOnly bool, alpineImage string, certMap map[string]pki.CertificatePKI) error {
	log.Infof(ctx, "[%s] Building up Controller Plane..", ControlRole)
	var errgrp errgroup.Group
	for _, host := range controlHosts {
		runHost := host
		if updateWorkersOnly {
			continue
		}
		errgrp.Go(func() error {
			return doDeployControlHost(ctx, runHost, localConnDialerFactory, prsMap, cpNodePlanMap[runHost.Address].Processes, alpineImage, certMap)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	log.Infof(ctx, "[%s] Successfully started Controller Plane..", ControlRole)
	return nil
}

func RemoveControlPlane(ctx context.Context, controlHosts []*hosts.Host, force bool) error {
	log.Infof(ctx, "[%s] Tearing down the Controller Plane..", ControlRole)
	for _, host := range controlHosts {
		// remove KubeAPI
		if err := removeKubeAPI(ctx, host); err != nil {
			return err
		}

		// remove KubeController
		if err := removeKubeController(ctx, host); err != nil {
			return nil
		}

		// remove scheduler
		err := removeScheduler(ctx, host)
		if err != nil {
			return err
		}

		// check if the host already is a worker
		if host.IsWorker {
			log.Infof(ctx, "[%s] Host [%s] is already a worker host, skipping delete kubelet and kubeproxy.", ControlRole, host.Address)
		} else {
			// remove KubeAPI
			if err := removeKubelet(ctx, host); err != nil {
				return err
			}
			// remove KubeController
			if err := removeKubeproxy(ctx, host); err != nil {
				return nil
			}
			// remove Sidekick
			if err := removeSidekick(ctx, host); err != nil {
				return err
			}
		}
	}
	log.Infof(ctx, "[%s] Successfully tore down Controller Plane..", ControlRole)
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
