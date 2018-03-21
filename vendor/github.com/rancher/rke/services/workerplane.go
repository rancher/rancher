package services

import (
	"context"

	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"golang.org/x/sync/errgroup"
)

const (
	unschedulableEtcdTaint = "node-role.kubernetes.io/etcd=true:NoExecute"
)

func RunWorkerPlane(ctx context.Context, allHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, processMap map[string]v3.Process, kubeletProcessHostMap map[*hosts.Host]v3.Process, certMap map[string]pki.CertificatePKI, updateWorkersOnly bool, alpineImage string) error {
	log.Infof(ctx, "[%s] Building up Worker Plane..", WorkerRole)
	var errgrp errgroup.Group
	for _, host := range allHosts {
		if updateWorkersOnly {
			if !host.UpdateWorker {
				continue
			}
		}
		if !host.IsControl && !host.IsWorker {
			// Add unschedulable taint
			host.ToAddTaints = append(host.ToAddTaints, unschedulableEtcdTaint)
		}
		runHost := host
		// maps are not thread safe
		hostProcessMap := copyProcessMap(processMap)
		errgrp.Go(func() error {
			hostProcessMap[KubeletContainerName] = kubeletProcessHostMap[runHost]
			return doDeployWorkerPlane(ctx, runHost, localConnDialerFactory, prsMap, hostProcessMap, certMap, alpineImage)
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

func copyProcessMap(m map[string]v3.Process) map[string]v3.Process {
	c := make(map[string]v3.Process)
	for k, v := range m {
		c[k] = v
	}
	return c
}
