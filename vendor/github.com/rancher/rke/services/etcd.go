package services

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"context"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/docker/docker/api/types/container"
	"github.com/pkg/errors"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

const (
	EtcdBackupPath  = "/opt/rke/etcdbackup/"
	EtcdRestorePath = "/opt/rke/etcdrestore/"
	EtcdDataDir     = "/var/lib/rancher/etcd/"
)

type EtcdBackup struct {
	// Enable or disable backup creation
	Backup bool
	// Creation period of the etcd backups
	Creation string
	// Retention period of the etcd backups
	Retention string
}

func RunEtcdPlane(
	ctx context.Context,
	etcdHosts []*hosts.Host,
	etcdNodePlanMap map[string]v3.RKEConfigNodePlan,
	localConnDialerFactory hosts.DialerFactory,
	prsMap map[string]v3.PrivateRegistry,
	updateWorkersOnly bool,
	alpineImage string,
	etcdBackup EtcdBackup) error {
	log.Infof(ctx, "[%s] Building up etcd plane..", ETCDRole)
	for _, host := range etcdHosts {
		if updateWorkersOnly {
			continue
		}
		etcdProcess := etcdNodePlanMap[host.Address].Processes[EtcdContainerName]
		imageCfg, hostCfg, _ := GetProcessConfig(etcdProcess)
		if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, EtcdContainerName, host.Address, ETCDRole, prsMap); err != nil {
			return err
		}
		if etcdBackup.Backup {
			if err := RunEtcdBackup(ctx, host, prsMap, alpineImage, etcdBackup.Creation, etcdBackup.Retention, EtcdBackupContainerName, false); err != nil {
				return err
			}
		}
		if err := createLogLink(ctx, host, EtcdContainerName, ETCDRole, alpineImage, prsMap); err != nil {
			return err
		}
	}
	log.Infof(ctx, "[%s] Successfully started etcd plane..", ETCDRole)
	return nil
}

func RemoveEtcdPlane(ctx context.Context, etcdHosts []*hosts.Host, force bool) error {
	log.Infof(ctx, "[%s] Tearing down etcd plane..", ETCDRole)
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
	log.Infof(ctx, "[%s] Successfully tore down etcd plane..", ETCDRole)
	return nil
}

func AddEtcdMember(ctx context.Context, toAddEtcdHost *hosts.Host, etcdHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory, cert, key []byte) error {
	log.Infof(ctx, "[add/%s] Adding member [etcd-%s] to etcd cluster", ETCDRole, toAddEtcdHost.HostnameOverride)
	peerURL := fmt.Sprintf("https://%s:2380", toAddEtcdHost.InternalAddress)
	added := false
	for _, host := range etcdHosts {
		if host.Address == toAddEtcdHost.Address {
			continue
		}
		etcdClient, err := getEtcdClient(ctx, host, localConnDialerFactory, cert, key)
		if err != nil {
			logrus.Debugf("Failed to create etcd client for host [%s]: %v", host.Address, err)
			continue
		}
		memAPI := etcdclient.NewMembersAPI(etcdClient)
		if _, err := memAPI.Add(ctx, peerURL); err != nil {
			logrus.Debugf("Failed to Add etcd member [%s] from host: %v", host.Address, err)
			continue
		}
		added = true
		break
	}
	if !added {
		return fmt.Errorf("Failed to add etcd member [etcd-%s] to etcd cluster", toAddEtcdHost.HostnameOverride)
	}
	log.Infof(ctx, "[add/%s] Successfully Added member [etcd-%s] to etcd cluster", ETCDRole, toAddEtcdHost.HostnameOverride)
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

func ReloadEtcdCluster(ctx context.Context, readyEtcdHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory, cert, key []byte, prsMap map[string]v3.PrivateRegistry, etcdNodePlanMap map[string]v3.RKEConfigNodePlan, alpineImage string) error {
	for _, etcdHost := range readyEtcdHosts {
		imageCfg, hostCfg, _ := GetProcessConfig(etcdNodePlanMap[etcdHost.Address].Processes[EtcdContainerName])
		if err := docker.DoRunContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdContainerName, etcdHost.Address, ETCDRole, prsMap); err != nil {
			return err
		}
		if err := createLogLink(ctx, etcdHost, EtcdContainerName, ETCDRole, alpineImage, prsMap); err != nil {
			return err
		}
	}
	time.Sleep(10 * time.Second)
	var healthy bool
	for _, host := range readyEtcdHosts {
		_, _, healthCheckURL := GetProcessConfig(etcdNodePlanMap[host.Address].Processes[EtcdContainerName])
		if healthy = isEtcdHealthy(ctx, localConnDialerFactory, host, cert, key, healthCheckURL); healthy {
			break
		}
	}
	if !healthy {
		return fmt.Errorf("[etcd] Etcd Cluster is not healthy")
	}
	return nil
}

func IsEtcdMember(ctx context.Context, etcdHost *hosts.Host, etcdHosts []*hosts.Host, localConnDialerFactory hosts.DialerFactory, cert, key []byte) (bool, error) {
	var listErr error
	peerURL := fmt.Sprintf("https://%s:2380", etcdHost.InternalAddress)
	for _, host := range etcdHosts {
		if host.Address == etcdHost.Address {
			continue
		}
		etcdClient, err := getEtcdClient(ctx, host, localConnDialerFactory, cert, key)
		if err != nil {
			listErr = errors.Wrapf(err, "Failed to create etcd client for host [%s]", host.Address)
			logrus.Debugf("Failed to create etcd client for host [%s]: %v", host.Address, err)
			continue
		}
		memAPI := etcdclient.NewMembersAPI(etcdClient)
		members, err := memAPI.List(ctx)
		if err != nil {
			listErr = errors.Wrapf(err, "Failed to create etcd client for host [%s]", host.Address)
			logrus.Debugf("Failed to list etcd cluster members [%s]: %v", etcdHost.Address, err)
			continue
		}
		for _, member := range members {
			if strings.Contains(member.PeerURLs[0], peerURL) {
				logrus.Infof("[etcd] member [%s] is already part of the etcd cluster", etcdHost.Address)
				return true, nil
			}
		}
		// reset the list of errors to handle new hosts
		listErr = nil
		break
	}
	if listErr != nil {
		return false, listErr
	}
	return false, nil
}

func RunEtcdBackup(ctx context.Context, etcdHost *hosts.Host, prsMap map[string]v3.PrivateRegistry, etcdBackupImage string, creation, retention, name string, once bool) error {
	log.Infof(ctx, "[etcd] Starting backup on host [%s]", etcdHost.Address)
	imageCfg := &container.Config{
		Cmd: []string{
			"/opt/rke/rke-etcd-backup",
			"rolling-backup",
			"--cacert", pki.GetCertPath(pki.CACertName),
			"--cert", pki.GetCertPath(pki.KubeNodeCertName),
			"--key", pki.GetKeyPath(pki.KubeNodeCertName),
			"--name", name,
			"--endpoints=" + etcdHost.InternalAddress + ":2379",
		},
		Image: etcdBackupImage,
	}
	if once {
		imageCfg.Cmd = append(imageCfg.Cmd, "--once")
	}
	if !once {
		imageCfg.Cmd = append(imageCfg.Cmd, "--retention="+retention)
		imageCfg.Cmd = append(imageCfg.Cmd, "--creation="+creation)
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/backup", EtcdBackupPath),
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(etcdHost.PrefixPath, "/etc/kubernetes"))},
		NetworkMode: container.NetworkMode("host"),
	}

	if once {
		if err := docker.DoRunContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdBackupOnceContainerName, etcdHost.Address, ETCDRole, prsMap); err != nil {
			return err
		}
		status, err := docker.WaitForContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdBackupOnceContainerName)
		if status != 0 || err != nil {
			return fmt.Errorf("Failed to take etcd backup exit code [%d]: %v", status, err)
		}
		return docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdBackupOnceContainerName)
	}
	return docker.DoRunContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdBackupContainerName, etcdHost.Address, ETCDRole, prsMap)
}

func RestoreEtcdBackup(ctx context.Context, etcdHost *hosts.Host, prsMap map[string]v3.PrivateRegistry, etcdRestoreImage, backupName, initCluster string) error {
	log.Infof(ctx, "[etcd] Restoring [%s] snapshot on etcd host [%s]", backupName, etcdHost.Address)
	nodeName := pki.GetEtcdCrtName(etcdHost.InternalAddress)
	backupPath := filepath.Join(EtcdBackupPath, backupName)

	imageCfg := &container.Config{
		Cmd: []string{
			"sh", "-c", strings.Join([]string{
				"/usr/local/bin/etcdctl",
				fmt.Sprintf("--endpoints=[%s:2379]", etcdHost.InternalAddress),
				"--cacert", pki.GetCertPath(pki.CACertName),
				"--cert", pki.GetCertPath(nodeName),
				"--key", pki.GetKeyPath(nodeName),
				"snapshot", "restore", backupPath,
				"--data-dir=" + EtcdRestorePath,
				"--name=etcd-" + etcdHost.HostnameOverride,
				"--initial-cluster=" + initCluster,
				"--initial-cluster-token=etcd-cluster-1",
				"--initial-advertise-peer-urls=https://" + etcdHost.InternalAddress + ":2380",
				"&& mv", EtcdRestorePath + "*", EtcdDataDir,
				"&& rm -rf", EtcdRestorePath,
			}, " "),
		},
		Env:   []string{"ETCDCTL_API=3"},
		Image: etcdRestoreImage,
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			"/opt/rke/:/opt/rke/:z",
			fmt.Sprintf("%s:/var/lib/rancher/etcd:z", path.Join(etcdHost.PrefixPath, "/var/lib/etcd")),
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(etcdHost.PrefixPath, "/etc/kubernetes"))},
		NetworkMode: container.NetworkMode("host"),
	}
	if err := docker.DoRunContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdRestoreContainerName, etcdHost.Address, ETCDRole, prsMap); err != nil {
		return err
	}
	status, err := docker.WaitForContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdRestoreContainerName)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("Failed to run etcd restore container, exit status is: %d", status)
	}
	return docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdRestoreContainerName)
}
