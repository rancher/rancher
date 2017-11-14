package services

import (
	"github.com/rancher/rke/hosts"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func RunControlPlane(controlHosts, etcdHosts []*hosts.Host, controlServices v3.RKEConfigServices, sidekickImage string) error {
	logrus.Infof("[%s] Building up Controller Plane..", ControlRole)
	for _, host := range controlHosts {

		if host.IsWorker {
			if err := removeNginxProxy(host); err != nil {
				return err
			}
		}
		// run sidekick
		if err := runSidekick(host, sidekickImage); err != nil {
			return err
		}
		// run kubeapi
		err := runKubeAPI(host, etcdHosts, controlServices.KubeAPI)
		if err != nil {
			return err
		}
		// run kubecontroller
		err = runKubeController(host, controlServices.KubeController)
		if err != nil {
			return err
		}
		// run scheduler
		err = runScheduler(host, controlServices.Scheduler)
		if err != nil {
			return err
		}
	}
	logrus.Infof("[%s] Successfully started Controller Plane..", ControlRole)
	return nil
}

func RemoveControlPlane(controlHosts []*hosts.Host, force bool) error {
	logrus.Infof("[%s] Tearing down the Controller Plane..", ControlRole)
	for _, host := range controlHosts {
		// remove KubeAPI
		if err := removeKubeAPI(host); err != nil {
			return err
		}

		// remove KubeController
		if err := removeKubeController(host); err != nil {
			return nil
		}

		// remove scheduler
		err := removeScheduler(host)
		if err != nil {
			return err
		}

		// check if the host already is a worker
		if host.IsWorker {
			logrus.Infof("[%s] Host [%s] is already a worker host, skipping delete kubelet and kubeproxy.", ControlRole, host.Address)
		} else {
			// remove KubeAPI
			if err := removeKubelet(host); err != nil {
				return err
			}
			// remove KubeController
			if err := removeKubeproxy(host); err != nil {
				return nil
			}
			// remove Sidekick
			if err := removeSidekick(host); err != nil {
				return err
			}
		}
	}
	logrus.Infof("[%s] Successfully teared down Controller Plane..", ControlRole)
	return nil
}
