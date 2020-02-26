package services

import (
	"context"

	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func runKubeAPI(ctx context.Context, host *hosts.Host, df hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry, kubeAPIProcess v3.Process, alpineImage string, certMap map[string]pki.CertificatePKI) error {
	imageCfg, hostCfg, healthCheckURL := GetProcessConfig(kubeAPIProcess, host)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, KubeAPIContainerName, host.Address, ControlRole, prsMap); err != nil {
		return err
	}
	if err := runHealthcheck(ctx, host, KubeAPIContainerName, df, healthCheckURL, certMap); err != nil {
		return err
	}
	return createLogLink(ctx, host, KubeAPIContainerName, ControlRole, alpineImage, prsMap)
}

func removeKubeAPI(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, KubeAPIContainerName, host.Address)
}

func RestartKubeAPI(ctx context.Context, host *hosts.Host) error {
	return docker.DoRestartContainer(ctx, host.DClient, KubeAPIContainerName, host.Address)
}

func RestartKubeAPIWithHealthcheck(ctx context.Context, hostList []*hosts.Host, df hosts.DialerFactory, certMap map[string]pki.CertificatePKI) error {
	log.Infof(ctx, "[%s] Restarting %s on contorl plane nodes..", ControlRole, KubeAPIContainerName)
	for _, runHost := range hostList {
		logrus.Debugf("[%s] Restarting %s on node [%s]", ControlRole, KubeAPIContainerName, runHost.Address)
		if err := RestartKubeAPI(ctx, runHost); err != nil {
			return err
		}
		logrus.Debugf("[%s] Running healthcheck for %s on node [%s]", ControlRole, KubeAPIContainerName, runHost.Address)
		if err := runHealthcheck(ctx, runHost, KubeAPIContainerName,
			df, GetHealthCheckURL(true, KubeAPIPort),
			certMap); err != nil {
			return err
		}
	}
	log.Infof(ctx, "[%s] Restarted %s on contorl plane nodes successfully", ControlRole, KubeAPIContainerName)
	return nil
}
