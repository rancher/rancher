package services

import (
	"context"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"golang.org/x/sync/errgroup"
)

func RunControlPlane(ctx context.Context, controlHosts, etcdHosts []*hosts.Host, controlServices v3.RKEConfigServices, sidekickImage, authorizationMode string, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry) error {
	log.Infof(ctx, "[%s] Building up Controller Plane..", ControlRole)
	var errgrp errgroup.Group
	for _, host := range controlHosts {
		runHost := host
		errgrp.Go(func() error {
			return doDeployControlHost(ctx, runHost, etcdHosts, controlServices, sidekickImage, authorizationMode, localConnDialerFactory, prsMap)
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

func doDeployControlHost(ctx context.Context, host *hosts.Host, etcdHosts []*hosts.Host, controlServices v3.RKEConfigServices, sidekickImage, authorizationMode string, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry) error {
	if host.IsWorker {
		if err := removeNginxProxy(ctx, host); err != nil {
			return err
		}
	}
	// run sidekick
	if err := runSidekick(ctx, host, sidekickImage, prsMap); err != nil {
		return err
	}
	// run kubeapi
	if err := runKubeAPI(ctx, host, etcdHosts, controlServices.KubeAPI, authorizationMode, localConnDialerFactory, prsMap); err != nil {
		return err
	}
	// run kubecontroller
	if err := runKubeController(ctx, host, controlServices.KubeController, authorizationMode, localConnDialerFactory, prsMap); err != nil {
		return err
	}
	// run scheduler
	return runScheduler(ctx, host, controlServices.Scheduler, localConnDialerFactory, prsMap)
}
