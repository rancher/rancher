package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"strings"
	"time"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/docker/docker/api/types/container"
	"github.com/pkg/errors"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/pki/cert"
	"github.com/rancher/rke/util"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	EtcdSnapshotPath         = "/opt/rke/etcd-snapshots/"
	EtcdRestorePath          = "/opt/rke/etcd-snapshots-restore/"
	EtcdDataDir              = "/var/lib/rancher/etcd/"
	EtcdInitWaitTime         = 10
	EtcdSnapshotWaitTime     = 5
	EtcdPermFixContainerName = "etcd-fix-perm"
)

func RunEtcdPlane(
	ctx context.Context,
	etcdHosts []*hosts.Host,
	etcdNodePlanMap map[string]v3.RKEConfigNodePlan,
	localConnDialerFactory hosts.DialerFactory,
	prsMap map[string]v3.PrivateRegistry,
	updateWorkersOnly bool,
	alpineImage string,
	es v3.ETCDService,
	certMap map[string]pki.CertificatePKI) error {
	log.Infof(ctx, "[%s] Building up etcd plane..", ETCDRole)
	for _, host := range etcdHosts {
		if updateWorkersOnly {
			continue
		}

		etcdProcess := etcdNodePlanMap[host.Address].Processes[EtcdContainerName]

		// need to run this first to set proper ownership and permissions on etcd data dir
		if err := setEtcdPermissions(ctx, host, prsMap, alpineImage, etcdProcess); err != nil {
			return err
		}
		imageCfg, hostCfg, _ := GetProcessConfig(etcdProcess, host)
		if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, EtcdContainerName, host.Address, ETCDRole, prsMap); err != nil {
			return err
		}
		if *es.Snapshot == true {
			rkeToolsImage, err := util.GetDefaultRKETools(alpineImage)
			if err != nil {
				return err
			}
			if err := RunEtcdSnapshotSave(ctx, host, prsMap, rkeToolsImage, EtcdSnapshotContainerName, false, es); err != nil {
				return err
			}
			if err := pki.SaveBackupBundleOnHost(ctx, host, rkeToolsImage, EtcdSnapshotPath, prsMap); err != nil {
				return err
			}
		} else {
			if err := docker.DoRemoveContainer(ctx, host.DClient, EtcdSnapshotContainerName, host.Address); err != nil {
				return err
			}
		}
		if err := createLogLink(ctx, host, EtcdContainerName, ETCDRole, alpineImage, prsMap); err != nil {
			return err
		}
	}
	log.Infof(ctx, "[%s] Successfully started etcd plane.. Checking etcd cluster health", ETCDRole)
	clientCert := cert.EncodeCertPEM(certMap[pki.KubeNodeCertName].Certificate)
	clientKey := cert.EncodePrivateKeyPEM(certMap[pki.KubeNodeCertName].Key)
	var healthError error
	var hosts []string
	for _, host := range etcdHosts {
		_, _, healthCheckURL := GetProcessConfig(etcdNodePlanMap[host.Address].Processes[EtcdContainerName], host)
		healthError = isEtcdHealthy(localConnDialerFactory, host, clientCert, clientKey, healthCheckURL)
		if healthError == nil {
			break
		}
		logrus.Warn(healthError)
		hosts = append(hosts, host.Address)
	}
	if healthError != nil {
		return fmt.Errorf("etcd cluster is unhealthy: hosts [%s] failed to report healthy."+
			" Check etcd container logs on each host for more information", strings.Join(hosts, ","))
	}
	return nil
}

func RestartEtcdPlane(ctx context.Context, etcdHosts []*hosts.Host) error {
	log.Infof(ctx, "[%s] Restarting up etcd plane..", ETCDRole)
	var errgrp errgroup.Group

	hostsQueue := util.GetObjectQueue(etcdHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				if err := docker.DoRestartContainer(ctx, runHost.DClient, EtcdContainerName, runHost.Address); err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	log.Infof(ctx, "[%s] Successfully restarted etcd plane..", ETCDRole)
	return nil
}

func RemoveEtcdPlane(ctx context.Context, etcdHosts []*hosts.Host, force bool) error {
	log.Infof(ctx, "[%s] Tearing down etcd plane..", ETCDRole)

	var errgrp errgroup.Group
	hostsQueue := util.GetObjectQueue(etcdHosts)
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				runHost := host.(*hosts.Host)
				if err := docker.DoRemoveContainer(ctx, runHost.DClient, EtcdContainerName, runHost.Address); err != nil {
					errList = append(errList, err)
				}
				if err := docker.DoRemoveContainer(ctx, runHost.DClient, EtcdSnapshotContainerName, runHost.Address); err != nil {
					errList = append(errList, err)
				}
				if !runHost.IsWorker || !runHost.IsControl || force {
					// remove unschedulable kubelet on etcd host
					if err := removeKubelet(ctx, runHost); err != nil {
						errList = append(errList, err)
					}
					if err := removeKubeproxy(ctx, runHost); err != nil {
						errList = append(errList, err)
					}
					if err := removeNginxProxy(ctx, runHost); err != nil {
						errList = append(errList, err)
					}
					if err := removeSidekick(ctx, runHost); err != nil {
						errList = append(errList, err)
					}
				}
			}
			return util.ErrList(errList)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
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

func ReloadEtcdCluster(ctx context.Context, readyEtcdHosts []*hosts.Host, newHost *hosts.Host, localConnDialerFactory hosts.DialerFactory, cert, key []byte, prsMap map[string]v3.PrivateRegistry, etcdNodePlanMap map[string]v3.RKEConfigNodePlan, alpineImage string) error {
	imageCfg, hostCfg, _ := GetProcessConfig(etcdNodePlanMap[newHost.Address].Processes[EtcdContainerName], newHost)

	if err := setEtcdPermissions(ctx, newHost, prsMap, alpineImage, etcdNodePlanMap[newHost.Address].Processes[EtcdContainerName]); err != nil {
		return err
	}

	if err := docker.DoRunContainer(ctx, newHost.DClient, imageCfg, hostCfg, EtcdContainerName, newHost.Address, ETCDRole, prsMap); err != nil {
		return err
	}
	if err := createLogLink(ctx, newHost, EtcdContainerName, ETCDRole, alpineImage, prsMap); err != nil {
		return err
	}
	time.Sleep(EtcdInitWaitTime * time.Second)
	var healthError error
	var hosts []string
	for _, host := range readyEtcdHosts {
		_, _, healthCheckURL := GetProcessConfig(etcdNodePlanMap[host.Address].Processes[EtcdContainerName], host)
		healthError = isEtcdHealthy(localConnDialerFactory, host, cert, key, healthCheckURL)
		if healthError == nil {
			break
		}
		logrus.Warn(healthError)
		hosts = append(hosts, host.Address)
	}
	if healthError != nil {
		return fmt.Errorf("etcd cluster is unhealthy: hosts [%s] failed to report healthy."+
			" Check etcd container logs on each host for more information", strings.Join(hosts, ","))
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

func RunEtcdSnapshotSave(ctx context.Context, etcdHost *hosts.Host, prsMap map[string]v3.PrivateRegistry, etcdSnapshotImage string, name string, once bool, es v3.ETCDService) error {
	backupCmd := "etcd-backup"
	restartPolicy := "always"
	imageCfg := &container.Config{
		Cmd: []string{
			"/opt/rke-tools/rke-etcd-backup",
			backupCmd,
			"save",
			"--cacert", pki.GetCertPath(pki.CACertName),
			"--cert", pki.GetCertPath(pki.KubeNodeCertName),
			"--key", pki.GetKeyPath(pki.KubeNodeCertName),
			"--name", name,
			"--endpoints=" + etcdHost.InternalAddress + ":2379",
		},
		Image: etcdSnapshotImage,
		Env:   es.ExtraEnv,
	}
	// Configure imageCfg for one time snapshot
	if once {
		imageCfg.Cmd = append(imageCfg.Cmd, "--once")
		restartPolicy = "no"
		// Configure imageCfg for rolling snapshots
	} else if es.BackupConfig == nil {
		imageCfg.Cmd = append(imageCfg.Cmd, "--retention="+es.Retention)
		imageCfg.Cmd = append(imageCfg.Cmd, "--creation="+es.Creation)
	}
	// Configure imageCfg for S3 backups
	if es.BackupConfig != nil {
		imageCfg = configS3BackupImgCmd(ctx, imageCfg, es.BackupConfig)
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/backup:z", EtcdSnapshotPath),
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(etcdHost.PrefixPath, "/etc/kubernetes"))},
		NetworkMode:   container.NetworkMode("host"),
		RestartPolicy: container.RestartPolicy{Name: restartPolicy},
	}

	if once {
		log.Infof(ctx, "[etcd] Running snapshot save once on host [%s]", etcdHost.Address)
		logrus.Debugf("[etcd] Using command [%s] for snapshot save once container [%s] on host [%s]", getSanitizedSnapshotCmd(imageCfg, es.BackupConfig), EtcdSnapshotOnceContainerName, etcdHost.Address)
		if err := docker.DoRemoveContainer(ctx, etcdHost.DClient, EtcdSnapshotOnceContainerName, etcdHost.Address); err != nil {
			return err
		}
		if err := docker.DoRunContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdSnapshotOnceContainerName, etcdHost.Address, ETCDRole, prsMap); err != nil {
			return err
		}
		status, _, stderr, err := docker.GetContainerOutput(ctx, etcdHost.DClient, EtcdSnapshotOnceContainerName, etcdHost.Address)
		if status != 0 || err != nil {
			if removeErr := docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdSnapshotOnceContainerName); removeErr != nil {
				log.Warnf(ctx, "[etcd] Failed to remove container [%s] on host [%s]: %v", removeErr, etcdHost.Address)
			}
			if err != nil {
				return err
			}
			return fmt.Errorf("[etcd] Failed to take one-time snapshot on host [%s], exit code [%d]: %v", etcdHost.Address, status, stderr)
		}

		return docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdSnapshotOnceContainerName)
	}
	log.Infof(ctx, "[etcd] Running rolling snapshot container [%s] on host [%s]", EtcdSnapshotOnceContainerName, etcdHost.Address)
	logrus.Debugf("[etcd] Using command [%s] for rolling snapshot container [%s] on host [%s]", getSanitizedSnapshotCmd(imageCfg, es.BackupConfig), EtcdSnapshotContainerName, etcdHost.Address)
	if err := docker.DoRemoveContainer(ctx, etcdHost.DClient, EtcdSnapshotContainerName, etcdHost.Address); err != nil {
		return err
	}
	if err := docker.DoRunContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdSnapshotContainerName, etcdHost.Address, ETCDRole, prsMap); err != nil {
		return err
	}
	// check if the container exited with error
	snapshotCont, err := docker.InspectContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdSnapshotContainerName)
	if err != nil {
		return err
	}
	time.Sleep(EtcdSnapshotWaitTime * time.Second)
	if snapshotCont.State.Status == "exited" || snapshotCont.State.Restarting {
		log.Warnf(ctx, "[etcd] etcd rolling snapshot container failed to start correctly")
		return docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdSnapshotContainerName)
	}
	return nil
}

func DownloadEtcdSnapshotFromS3(ctx context.Context, etcdHost *hosts.Host, prsMap map[string]v3.PrivateRegistry, etcdSnapshotImage string, name string, es v3.ETCDService) error {
	s3Backend := es.BackupConfig.S3BackupConfig
	if len(s3Backend.Endpoint) == 0 || len(s3Backend.BucketName) == 0 {
		return fmt.Errorf("failed to get snapshot [%s] from s3 on host [%s], invalid s3 configurations", name, etcdHost.Address)
	}
	imageCfg := &container.Config{
		Cmd: []string{
			"/opt/rke-tools/rke-etcd-backup",
			"etcd-backup",
			"download",
			"--name", name,
			"--s3-backup=true",
			"--s3-endpoint=" + s3Backend.Endpoint,
			"--s3-accessKey=" + s3Backend.AccessKey,
			"--s3-secretKey=" + s3Backend.SecretKey,
			"--s3-bucketName=" + s3Backend.BucketName,
			"--s3-region=" + s3Backend.Region,
		},
		Image: etcdSnapshotImage,
		Env:   es.ExtraEnv,
	}
	s3Logline := fmt.Sprintf("[etcd] Snapshot [%s] will be downloaded on host [%s] from S3 compatible backend at [%s] from bucket [%s] using accesskey [%s]", name, etcdHost.Address, s3Backend.Endpoint, s3Backend.BucketName, s3Backend.AccessKey)
	if s3Backend.Region != "" {
		s3Logline += fmt.Sprintf(" and using region [%s]", s3Backend.Region)
	}

	if s3Backend.CustomCA != "" {
		caStr := base64.StdEncoding.EncodeToString([]byte(s3Backend.CustomCA))
		imageCfg.Cmd = append(imageCfg.Cmd, "--s3-endpoint-ca="+caStr)
		s3Logline += fmt.Sprintf(" and using endpoint CA [%s]", caStr)
	}
	if s3Backend.Folder != "" {
		imageCfg.Cmd = append(imageCfg.Cmd, "--s3-folder="+s3Backend.Folder)
		s3Logline += fmt.Sprintf(" and using folder [%s]", s3Backend.Folder)
	}
	log.Infof(ctx, s3Logline)
	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/backup:z", EtcdSnapshotPath),
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(etcdHost.PrefixPath, "/etc/kubernetes"))},
		NetworkMode:   container.NetworkMode("host"),
		RestartPolicy: container.RestartPolicy{Name: "no"},
	}
	if err := docker.DoRemoveContainer(ctx, etcdHost.DClient, EtcdDownloadBackupContainerName, etcdHost.Address); err != nil {
		return err
	}
	if err := docker.DoRunContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdDownloadBackupContainerName, etcdHost.Address, ETCDRole, prsMap); err != nil {
		return err
	}

	status, _, stderr, err := docker.GetContainerOutput(ctx, etcdHost.DClient, EtcdDownloadBackupContainerName, etcdHost.Address)
	if status != 0 || err != nil {
		if removeErr := docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdDownloadBackupContainerName); removeErr != nil {
			log.Warnf(ctx, "Failed to remove container [%s]: %v", removeErr)
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("Failed to download etcd snapshot from s3, exit code [%d]: %v", status, stderr)
	}
	return docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdDownloadBackupContainerName)
}

func RestoreEtcdSnapshot(ctx context.Context, etcdHost *hosts.Host, prsMap map[string]v3.PrivateRegistry,
	etcdRestoreImage, etcdBackupImage, snapshotName, initCluster string, es v3.ETCDService) error {
	log.Infof(ctx, "[etcd] Restoring [%s] snapshot on etcd host [%s]", snapshotName, etcdHost.Address)
	nodeName := pki.GetCrtNameForHost(etcdHost, pki.EtcdCertName)
	snapshotPath := fmt.Sprintf("%s%s", EtcdSnapshotPath, snapshotName)

	// make sure that restore path is empty otherwise etcd restore will fail
	imageCfg := &container.Config{
		Cmd: []string{
			"sh", "-c", strings.Join([]string{
				"rm -rf", EtcdRestorePath,
				"&& /usr/local/bin/etcdctl",
				fmt.Sprintf("--endpoints=[%s:2379]", etcdHost.InternalAddress),
				"--cacert", pki.GetCertPath(pki.CACertName),
				"--cert", pki.GetCertPath(nodeName),
				"--key", pki.GetKeyPath(nodeName),
				"snapshot", "restore", snapshotPath,
				"--data-dir=" + EtcdRestorePath,
				"--name=etcd-" + etcdHost.HostnameOverride,
				"--initial-cluster=" + initCluster,
				"--initial-cluster-token=etcd-cluster-1",
				"--initial-advertise-peer-urls=https://" + etcdHost.InternalAddress + ":2380",
				"&& mv", EtcdRestorePath + "*", EtcdDataDir,
				"&& rm -rf", EtcdRestorePath,
			}, " "),
		},
		Env:   append([]string{"ETCDCTL_API=3"}, es.ExtraEnv...),
		Image: etcdRestoreImage,
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			"/opt/rke/:/opt/rke/:z",
			fmt.Sprintf("%s:/var/lib/rancher/etcd:z", path.Join(etcdHost.PrefixPath, "/var/lib/etcd")),
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(etcdHost.PrefixPath, "/etc/kubernetes"))},
		NetworkMode: container.NetworkMode("host"),
	}
	if err := docker.DoRemoveContainer(ctx, etcdHost.DClient, EtcdRestoreContainerName, etcdHost.Address); err != nil {
		return err
	}
	if err := docker.DoRunContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdRestoreContainerName, etcdHost.Address, ETCDRole, prsMap); err != nil {
		return err
	}
	status, err := docker.WaitForContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdRestoreContainerName)
	if err != nil {
		return err
	}
	if status != 0 {
		containerLog, _, err := docker.GetContainerLogsStdoutStderr(ctx, etcdHost.DClient, EtcdRestoreContainerName, "5", false)
		if err != nil {
			return err
		}
		if err := docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdRestoreContainerName); err != nil {
			return err
		}
		// printing the restore container's logs
		return fmt.Errorf("Failed to run etcd restore container, exit status is: %d, container logs: %s", status, containerLog)
	}
	if err := docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdRestoreContainerName); err != nil {
		return err
	}
	return RunEtcdSnapshotRemove(ctx, etcdHost, prsMap, etcdBackupImage, snapshotName, true, es)
}

func RunEtcdSnapshotRemove(ctx context.Context, etcdHost *hosts.Host, prsMap map[string]v3.PrivateRegistry, etcdSnapshotImage string, name string, cleanupRestore bool, es v3.ETCDService) error {
	log.Infof(ctx, "[etcd] Removing snapshot [%s] from host [%s]", name, etcdHost.Address)
	imageCfg := &container.Config{
		Image: etcdSnapshotImage,
		Env:   es.ExtraEnv,
		Cmd: []string{
			"/opt/rke-tools/rke-etcd-backup",
			"etcd-backup",
			"delete",
			"--name", name,
		},
	}
	if cleanupRestore {
		imageCfg.Cmd = append(imageCfg.Cmd, "--cleanup")
	}
	if es.BackupConfig != nil && es.BackupConfig.S3BackupConfig != nil {
		s3cmd := []string{
			"--s3-backup",
			"--s3-endpoint=" + es.BackupConfig.S3BackupConfig.Endpoint,
			"--s3-accessKey=" + es.BackupConfig.S3BackupConfig.AccessKey,
			"--s3-secretKey=" + es.BackupConfig.S3BackupConfig.SecretKey,
			"--s3-bucketName=" + es.BackupConfig.S3BackupConfig.BucketName,
			"--s3-region=" + es.BackupConfig.S3BackupConfig.Region,
		}
		if es.BackupConfig.S3BackupConfig.CustomCA != "" {
			caStr := base64.StdEncoding.EncodeToString([]byte(es.BackupConfig.S3BackupConfig.CustomCA))
			s3cmd = append(s3cmd, "--s3-endpoint-ca="+caStr)
		}
		if es.BackupConfig.S3BackupConfig.Folder != "" {
			s3cmd = append(s3cmd, "--s3-folder="+es.BackupConfig.S3BackupConfig.Folder)
		}
		imageCfg.Cmd = append(imageCfg.Cmd, s3cmd...)
	}

	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/backup:z", EtcdSnapshotPath),
		},
		RestartPolicy: container.RestartPolicy{Name: "no"},
	}
	if err := docker.DoRemoveContainer(ctx, etcdHost.DClient, EtcdSnapshotRemoveContainerName, etcdHost.Address); err != nil {
		return err
	}
	if err := docker.DoRunContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdSnapshotRemoveContainerName, etcdHost.Address, ETCDRole, prsMap); err != nil {
		return err
	}
	status, _, stderr, err := docker.GetContainerOutput(ctx, etcdHost.DClient, EtcdSnapshotRemoveContainerName, etcdHost.Address)
	if status != 0 || err != nil {
		if removeErr := docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdSnapshotRemoveContainerName); removeErr != nil {
			log.Warnf(ctx, "Failed to remove container [%s]: %v", removeErr)
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("Failed to remove snapshot [%s] on host [%s], exit code [%d]: %v", EtcdSnapshotRemoveContainerName, etcdHost.Address, status, stderr)
	}

	return docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdSnapshotRemoveContainerName)
}

func GetEtcdSnapshotChecksum(ctx context.Context, etcdHost *hosts.Host, prsMap map[string]v3.PrivateRegistry, alpineImage, snapshotName string) (string, error) {
	var checksum string
	var err error
	var stderr string

	// compressedSnapshotPath := fmt.Sprintf("%s%s.%s", EtcdSnapshotPath, snapshotName, EtcdSnapshotCompressedExtension)
	snapshotPath := fmt.Sprintf("%s%s", EtcdSnapshotPath, snapshotName)
	imageCfg := &container.Config{
		Cmd: []string{
			"sh", "-c", strings.Join([]string{
				" if [ -f '", snapshotPath, "' ]; then md5sum '", snapshotPath, "' | cut -f1 -d' ' | tr -d '\n'; else echo 'snapshot file does not exist' >&2; fi"}, ""),
		},
		Image: alpineImage,
	}
	hostCfg := &container.HostConfig{
		Binds: []string{
			"/opt/rke/:/opt/rke/:z",
		}}

	if err := docker.DoRunContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdChecksumContainerName, etcdHost.Address, ETCDRole, prsMap); err != nil {
		return checksum, err
	}
	if _, err := docker.WaitForContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdChecksumContainerName); err != nil {
		return checksum, err
	}
	stderr, checksum, err = docker.GetContainerLogsStdoutStderr(ctx, etcdHost.DClient, EtcdChecksumContainerName, "1", false)
	if err != nil {
		return checksum, err
	}
	if stderr != "" {
		return checksum, fmt.Errorf("Error output not nil from snapshot checksum container [%s]: %s", EtcdChecksumContainerName, stderr)

	}
	if err := docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdChecksumContainerName); err != nil {
		return checksum, err
	}
	return checksum, nil
}

func configS3BackupImgCmd(ctx context.Context, imageCfg *container.Config, bc *v3.BackupConfig) *container.Config {
	cmd := []string{
		"--creation=" + fmt.Sprintf("%dh", bc.IntervalHours),
		"--retention=" + fmt.Sprintf("%dh", bc.Retention*bc.IntervalHours),
	}

	if bc.S3BackupConfig != nil {
		cmd = append(cmd, []string{
			"--s3-backup=true",
			"--s3-endpoint=" + bc.S3BackupConfig.Endpoint,
			"--s3-accessKey=" + bc.S3BackupConfig.AccessKey,
			"--s3-secretKey=" + bc.S3BackupConfig.SecretKey,
			"--s3-bucketName=" + bc.S3BackupConfig.BucketName,
			"--s3-region=" + bc.S3BackupConfig.Region,
		}...)
		s3Logline := fmt.Sprintf("[etcd] Snapshots configured to S3 compatible backend at [%s] to bucket [%s] using accesskey [%s]", bc.S3BackupConfig.Endpoint, bc.S3BackupConfig.BucketName, bc.S3BackupConfig.AccessKey)
		if bc.S3BackupConfig.Region != "" {
			s3Logline += fmt.Sprintf(" and using region [%s]", bc.S3BackupConfig.Region)
		}
		if bc.S3BackupConfig.CustomCA != "" {
			caStr := base64.StdEncoding.EncodeToString([]byte(bc.S3BackupConfig.CustomCA))
			cmd = append(cmd, "--s3-endpoint-ca="+caStr)
			s3Logline += fmt.Sprintf(" and using endpoint CA [%s]", caStr)
		}
		if bc.S3BackupConfig.Folder != "" {
			cmd = append(cmd, "--s3-folder="+bc.S3BackupConfig.Folder)
			s3Logline += fmt.Sprintf(" and using folder [%s]", bc.S3BackupConfig.Folder)
		}
		log.Infof(ctx, s3Logline)
	}
	imageCfg.Cmd = append(imageCfg.Cmd, cmd...)
	return imageCfg
}

func StartBackupServer(ctx context.Context, etcdHost *hosts.Host, prsMap map[string]v3.PrivateRegistry, etcdSnapshotImage string, name string) error {
	log.Infof(ctx, "[etcd] starting backup server on host [%s]", etcdHost.Address)

	imageCfg := &container.Config{
		Cmd: []string{
			"/opt/rke-tools/rke-etcd-backup",
			"etcd-backup",
			"serve",
			"--name", name,
			"--cacert", pki.GetCertPath(pki.CACertName),
			"--cert", pki.GetCertPath(pki.KubeNodeCertName),
			"--key", pki.GetKeyPath(pki.KubeNodeCertName),
		},
		Image: etcdSnapshotImage,
	}

	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/backup:z", EtcdSnapshotPath),
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(etcdHost.PrefixPath, "/etc/kubernetes"))},
		NetworkMode:   container.NetworkMode("host"),
		RestartPolicy: container.RestartPolicy{Name: "no"},
	}
	if err := docker.DoRemoveContainer(ctx, etcdHost.DClient, EtcdServeBackupContainerName, etcdHost.Address); err != nil {
		return err
	}
	if err := docker.DoRunContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdServeBackupContainerName, etcdHost.Address, ETCDRole, prsMap); err != nil {
		return err
	}
	time.Sleep(EtcdSnapshotWaitTime * time.Second)
	container, err := docker.InspectContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdServeBackupContainerName)
	if err != nil {
		return err
	}
	if !container.State.Running {
		containerLog, _, err := docker.GetContainerLogsStdoutStderr(ctx, etcdHost.DClient, EtcdServeBackupContainerName, "1", false)
		if err != nil {
			return err
		}
		if err := docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdServeBackupContainerName); err != nil {
			return err
		}
		// printing the restore container's logs
		return fmt.Errorf("Failed to run backup server container, container logs: %s", containerLog)
	}
	return nil
}

func DownloadEtcdSnapshotFromBackupServer(ctx context.Context, etcdHost *hosts.Host, prsMap map[string]v3.PrivateRegistry, etcdSnapshotImage, name string, backupServer *hosts.Host) error {
	log.Infof(ctx, "[etcd] Get snapshot [%s] on host [%s]", name, etcdHost.Address)
	imageCfg := &container.Config{
		Cmd: []string{
			"/opt/rke-tools/rke-etcd-backup",
			"etcd-backup",
			"download",
			"--name", name,
			"--local-endpoint", backupServer.InternalAddress,
			"--cacert", pki.GetCertPath(pki.CACertName),
			"--cert", pki.GetCertPath(pki.KubeNodeCertName),
			"--key", pki.GetKeyPath(pki.KubeNodeCertName),
		},
		Image: etcdSnapshotImage,
	}

	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/backup:z", EtcdSnapshotPath),
			fmt.Sprintf("%s:/etc/kubernetes:z", path.Join(etcdHost.PrefixPath, "/etc/kubernetes"))},
		NetworkMode:   container.NetworkMode("host"),
		RestartPolicy: container.RestartPolicy{Name: "on-failure"},
	}
	if err := docker.DoRemoveContainer(ctx, etcdHost.DClient, EtcdDownloadBackupContainerName, etcdHost.Address); err != nil {
		return err
	}
	if err := docker.DoRunContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdDownloadBackupContainerName, etcdHost.Address, ETCDRole, prsMap); err != nil {
		return err
	}

	status, _, stderr, err := docker.GetContainerOutput(ctx, etcdHost.DClient, EtcdDownloadBackupContainerName, etcdHost.Address)
	if status != 0 || err != nil {
		if removeErr := docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdDownloadBackupContainerName); removeErr != nil {
			log.Warnf(ctx, "Failed to remove container [%s]: %v", removeErr)
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("Failed to download etcd snapshot from backup server [%s], exit code [%d]: %v", backupServer.Address, status, stderr)
	}
	return docker.RemoveContainer(ctx, etcdHost.DClient, etcdHost.Address, EtcdDownloadBackupContainerName)
}

func setEtcdPermissions(ctx context.Context, etcdHost *hosts.Host, prsMap map[string]v3.PrivateRegistry, alpineImage string, process v3.Process) error {
	var dataBind string

	cmd := fmt.Sprintf("chmod 700 %s", EtcdDataDir)
	if len(process.User) != 0 {
		cmd = fmt.Sprintf("chmod 700 %s ; chown -R %s %s", EtcdDataDir, process.User, EtcdDataDir)
	}
	imageCfg := &container.Config{
		Cmd: []string{
			"sh", "-c",
			cmd,
		},
		Image: alpineImage,
	}
	for _, bind := range process.Binds {
		if strings.Contains(bind, "/var/lib/etcd") {
			dataBind = bind
		}
	}
	hostCfg := &container.HostConfig{
		Binds: []string{dataBind},
	}
	if err := docker.DoRunOnetimeContainer(ctx, etcdHost.DClient, imageCfg, hostCfg, EtcdPermFixContainerName,
		etcdHost.Address, ETCDRole, prsMap); err != nil {
		return err
	}
	return docker.DoRemoveContainer(ctx, etcdHost.DClient, EtcdPermFixContainerName, etcdHost.Address)
}

func getSanitizedSnapshotCmd(imageCfg *container.Config, bc *v3.BackupConfig) string {
	cmd := strings.Join(imageCfg.Cmd, " ")
	if bc != nil && bc.S3BackupConfig != nil {
		return strings.Replace(cmd, bc.S3BackupConfig.SecretKey, "***", -1)
	}
	return cmd
}
