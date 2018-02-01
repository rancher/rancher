package services

import (
	"fmt"
	"time"

	"context"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/docker/docker/api/types/container"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func RunEtcdPlane(ctx context.Context, etcdHosts []*hosts.Host, etcdService v3.ETCDService, localConnDialerFactory hosts.DialerFactory, prsMap map[string]v3.PrivateRegistry) error {
	log.Infof(ctx, "[%s] Building up Etcd Plane..", ETCDRole)
	initCluster := getEtcdInitialCluster(etcdHosts)
	for _, host := range etcdHosts {

		nodeName := pki.GetEtcdCrtName(host.InternalAddress)
		imageCfg, hostCfg := buildEtcdConfig(host, etcdService, initCluster, nodeName)
		err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, EtcdContainerName, host.Address, ETCDRole, prsMap)
		if err != nil {
			return err
		}
	}
	log.Infof(ctx, "[%s] Successfully started Etcd Plane..", ETCDRole)
	return nil
}

func RemoveEtcdPlane(ctx context.Context, etcdHosts []*hosts.Host, force bool) error {
	log.Infof(ctx, "[%s] Tearing down Etcd Plane..", ETCDRole)
	for _, host := range etcdHosts {
		err := docker.DoRemoveContainer(ctx, host.DClient, EtcdContainerName, host.Address)
		if err != nil {
			return err
		}
		if !host.IsWorker || !host.IsControl || force {
			// remove unschedulable kubelet on etcd host
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
		}

	}
	log.Infof(ctx, "[%s] Successfully tore down Etcd Plane..", ETCDRole)
	return nil
}

func buildEtcdConfig(host *hosts.Host, etcdService v3.ETCDService, initCluster, nodeName string) (*container.Config, *container.HostConfig) {
	clusterState := "new"
	if host.ExistingEtcdCluster {
		clusterState = "existing"
	}
	imageCfg := &container.Config{
		Image: etcdService.Image,
		Cmd: []string{"/usr/local/bin/etcd",
			"--name=etcd-" + host.HostnameOverride,
			"--data-dir=/etcd-data",
			"--advertise-client-urls=https://" + host.InternalAddress + ":2379,https://" + host.InternalAddress + ":4001",
			"--listen-client-urls=https://0.0.0.0:2379",
			"--initial-advertise-peer-urls=https://" + host.InternalAddress + ":2380",
			"--listen-peer-urls=https://0.0.0.0:2380",
			"--initial-cluster-token=etcd-cluster-1",
			"--initial-cluster=" + initCluster,
			"--initial-cluster-state=" + clusterState,
			"--peer-client-cert-auth",
			"--client-cert-auth",
			"--trusted-ca-file=" + pki.GetCertPath(pki.CACertName),
			"--peer-trusted-ca-file=" + pki.GetCertPath(pki.CACertName),
			"--cert-file=" + pki.GetCertPath(nodeName),
			"--key-file=" + pki.GetKeyPath(nodeName),
			"--peer-cert-file=" + pki.GetCertPath(nodeName),
			"--peer-key-file=" + pki.GetKeyPath(nodeName),
		},
	}
	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "always"},
		Binds: []string{
			"/var/lib/etcd:/etcd-data",
			"/etc/kubernetes:/etc/kubernetes",
		},
		NetworkMode: "host",
	}
	for arg, value := range etcdService.ExtraArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		imageCfg.Entrypoint = append(imageCfg.Entrypoint, cmd)
	}

	return imageCfg, hostCfg
}

func AddEtcdMember(ctx context.Context, etcdHost *hosts.Host, etcdHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory, cert, key []byte) error {
	log.Infof(ctx, "[add/%s] Adding member [etcd-%s] to etcd cluster", ETCDRole, etcdHost.HostnameOverride)
	peerURL := fmt.Sprintf("https://%s:2380", etcdHost.InternalAddress)
	added := false
	for _, host := range etcdHosts {
		etcdClient, err := getEtcdClient(ctx, host, localConnDialerFactory, cert, key)
		if err != nil {
			logrus.Debugf("Failed to create etcd client for host [%s]: %v", host.Address, err)
			continue
		}
		memAPI := etcdclient.NewMembersAPI(etcdClient)
		if _, err := memAPI.Add(ctx, peerURL); err != nil {
			logrus.Debugf("Failed to list etcd members from host [%s]: %v", host.Address, err)
			continue
		}
		added = true
		break
	}
	if !added {
		return fmt.Errorf("Failed to add etcd member [etcd-%s] from etcd cluster", etcdHost.HostnameOverride)
	}
	log.Infof(ctx, "[add/%s] Successfully Added member [etcd-%s] to etcd cluster", ETCDRole, etcdHost.HostnameOverride)
	return nil
}

func RemoveEtcdMember(ctx context.Context, etcdHost *hosts.Host, etcdHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory, cert, key []byte) error {
	log.Infof(ctx, "[remove/%s] Removing member [etcd-%s] from etcd cluster", ETCDRole, etcdHost.HostnameOverride)
	var mID string
	removed := false
	for _, host := range etcdHosts {
		etcdClient, err := getEtcdClient(ctx, host, localConnDialerFactory, cert, key)
		if err != nil {
			logrus.Debugf("Failed to create etcd client for host [%s]: %v", host.Address, err)
			continue
		}
		memAPI := etcdclient.NewMembersAPI(etcdClient)
		members, err := memAPI.List(ctx)
		if err != nil {
			logrus.Debugf("Failed to list etcd members from host [%s]: %v", host.Address, err)
			continue
		}
		for _, member := range members {
			if member.Name == fmt.Sprintf("etcd-%s", etcdHost.HostnameOverride) {
				mID = member.ID
				break
			}
		}
		if err := memAPI.Remove(ctx, mID); err != nil {
			logrus.Debugf("Failed to list etcd members from host [%s]: %v", host.Address, err)
			continue
		}
		removed = true
		break
	}
	if !removed {
		return fmt.Errorf("Failed to delete etcd member [etcd-%s] from etcd cluster", etcdHost.HostnameOverride)
	}
	log.Infof(ctx, "[remove/%s] Successfully removed member [etcd-%s] from etcd cluster", ETCDRole, etcdHost.HostnameOverride)
	return nil
}

func ReloadEtcdCluster(ctx context.Context, etcdHosts []*hosts.Host, etcdService v3.ETCDService, localConnDialerFactory hosts.DialerFactory, cert, key []byte, prsMap map[string]v3.PrivateRegistry) error {
	readyEtcdHosts := []*hosts.Host{}
	for _, host := range etcdHosts {
		if !host.ToAddEtcdMember {
			readyEtcdHosts = append(readyEtcdHosts, host)
			host.ExistingEtcdCluster = true
		}
	}
	initCluster := getEtcdInitialCluster(readyEtcdHosts)
	for _, host := range readyEtcdHosts {
		imageCfg, hostCfg := buildEtcdConfig(host, etcdService, initCluster, pki.GetEtcdCrtName(host.InternalAddress))
		if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, EtcdContainerName, host.Address, ETCDRole, prsMap); err != nil {
			return err
		}
	}
	time.Sleep(10 * time.Second)
	var healthy bool
	for _, host := range readyEtcdHosts {
		if healthy = isEtcdHealthy(ctx, localConnDialerFactory, host, cert, key); healthy {
			break
		}
	}
	if !healthy {
		return fmt.Errorf("[etcd] Etcd Cluster is not healthy")
	}
	return nil
}
