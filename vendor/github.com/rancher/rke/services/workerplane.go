package services

import (
	"github.com/rancher/rke/hosts"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func RunWorkerPlane(controlHosts, workerHosts []*hosts.Host, workerServices v3.RKEConfigServices, nginxProxyImage, sidekickImage string) error {
	logrus.Infof("[%s] Building up Worker Plane..", WorkerRole)
	for _, host := range controlHosts {
		// run sidekick
		if err := runSidekick(host, sidekickImage); err != nil {
			return err
		}
		// run kubelet
		// only one master for now
		if err := runKubelet(host, workerServices.Kubelet); err != nil {
			return err
		}
		if err := runKubeproxy(host, workerServices.Kubeproxy); err != nil {
			return err
		}
	}
	for _, host := range workerHosts {
		// run nginx proxy
		if !host.IsControl {
			if err := runNginxProxy(host, controlHosts, nginxProxyImage); err != nil {
				return err
			}
		}
		// run sidekick
		if err := runSidekick(host, sidekickImage); err != nil {
			return err
		}
		// run kubelet
		if err := runKubelet(host, workerServices.Kubelet); err != nil {
			return err
		}
		// run kubeproxy
		if err := runKubeproxy(host, workerServices.Kubeproxy); err != nil {
			return err
		}
	}
	logrus.Infof("[%s] Successfully started Worker Plane..", WorkerRole)
	return nil
}

func RemoveWorkerPlane(workerHosts []*hosts.Host, force bool) error {
	logrus.Infof("[%s] Tearing down Worker Plane..", WorkerRole)
	for _, host := range workerHosts {
		// check if the host already is a controlplane
		if host.IsControl && !force {
			logrus.Infof("[%s] Host [%s] is already a controlplane host, nothing to do.", WorkerRole, host.Address)
			return nil
		}

		if err := removeKubelet(host); err != nil {
			return err
		}
		if err := removeKubeproxy(host); err != nil {
			return err
		}
		if err := removeNginxProxy(host); err != nil {
			return err
		}
		if err := removeSidekick(host); err != nil {
			return err
		}
		logrus.Infof("[%s] Successfully teared down Worker Plane..", WorkerRole)
	}

	return nil
}
