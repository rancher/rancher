package services

import (
	"context"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"golang.org/x/sync/errgroup"
)

const (
	unschedulableEtcdTaint = "node-role.kubernetes.io/etcd=true:NoExecute"
)

func RunWorkerPlane(ctx context.Context, controlHosts, workerHosts, etcdHosts []*hosts.Host, workerServices v3.RKEConfigServices, nginxProxyImage, sidekickImage string, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry) error {
	log.Infof(ctx, "[%s] Building up Worker Plane..", WorkerRole)
	var errgrp errgroup.Group

	// Deploy worker components on etcd hosts
	for _, host := range etcdHosts {
		if !host.IsControl && !host.IsWorker {
			// Add unschedulable taint
			host.ToAddTaints = append(host.ToAddTaints, unschedulableEtcdTaint)
		}
		etcdHost := host
		errgrp.Go(func() error {
			return doDeployWorkerPlane(ctx, etcdHost, workerServices, nginxProxyImage, sidekickImage, localConnDialerFactory, controlHosts, prsMap)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}

	// Deploy worker components on control hosts
	for _, host := range controlHosts {
		controlHost := host
		errgrp.Go(func() error {
			return doDeployWorkerPlane(ctx, controlHost, workerServices, nginxProxyImage, sidekickImage, localConnDialerFactory, controlHosts, prsMap)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	// Deploy worker components on worker hosts
	for _, host := range workerHosts {
		workerHost := host
		errgrp.Go(func() error {
			return doDeployWorkerPlane(ctx, workerHost, workerServices, nginxProxyImage, sidekickImage, localConnDialerFactory, controlHosts, prsMap)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	log.Infof(ctx, "[%s] Successfully started Worker Plane..", WorkerRole)
	return nil
}

func RemoveWorkerPlane(ctx context.Context, workerHosts []*hosts.Host, force bool) error {
	log.Infof(ctx, "[%s] Tearing down Worker Plane..", WorkerRole)
	for _, host := range workerHosts {
		// check if the host already is a controlplane
		if host.IsControl && !force {
			log.Infof(ctx, "[%s] Host [%s] is already a controlplane host, nothing to do.", WorkerRole, host.Address)
			return nil
		}

		if err := removeKubelet(ctx, host); err != nil {
			return err
		}
		if err := removeKubeproxy(ctx, host); err != nil {
			return err
		}
		if err := removeNginxProxy(ctx, host); err != nil {
			return err
		}
		if err := removeSidekick(ctx, host); err != nil {
			return err
		}
		log.Infof(ctx, "[%s] Successfully tore down Worker Plane..", WorkerRole)
	}

	return nil
}

func doDeployWorkerPlane(ctx context.Context, host *hosts.Host,
	workerServices v3.RKEConfigServices,
	nginxProxyImage, sidekickImage string,
	localConnDialerFactory hosts.DialerFactory,
	controlHosts []*hosts.Host,
	prsMap map[string]v3.PrivateRegistry) error {

	// run nginx proxy
	if !host.IsControl {
		if err := runNginxProxy(ctx, host, controlHosts, nginxProxyImage, prsMap); err != nil {
			return err
		}
	}
	// run sidekick
	if err := runSidekick(ctx, host, sidekickImage, prsMap); err != nil {
		return err
	}
	// run kubelet
	if err := runKubelet(ctx, host, workerServices.Kubelet, localConnDialerFactory, prsMap); err != nil {
		return err
	}
	return runKubeproxy(ctx, host, workerServices.Kubeproxy, localConnDialerFactory, prsMap)
}
