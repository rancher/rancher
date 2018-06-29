package cluster

import (
	"context"

	"github.com/rancher/rke/hosts"
	"golang.org/x/sync/errgroup"
)

func (c *Cluster) CleanDeadLogs(ctx context.Context) error {
	hostList := hosts.GetUniqueHostList(c.EtcdHosts, c.ControlPlaneHosts, c.WorkerHosts)

	var errgrp errgroup.Group

	for _, host := range hostList {
		if !host.UpdateWorker {
			continue
		}
		runHost := host
		errgrp.Go(func() error {
			return hosts.DoRunLogCleaner(ctx, runHost, c.SystemImages.Alpine, c.PrivateRegistriesMap)
		})
	}
	return errgrp.Wait()
}
