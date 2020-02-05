package cluster

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/rke/util"
	"golang.org/x/sync/errgroup"
)

func (c *Cluster) SnapshotEtcd(ctx context.Context, snapshotName string) error {
	backupImage := c.getBackupImage()
	for _, host := range c.EtcdHosts {
		if err := services.RunEtcdSnapshotSave(ctx, host, c.PrivateRegistriesMap, backupImage, snapshotName, true, c.Services.Etcd); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cluster) DeployRestoreCerts(ctx context.Context, clusterCerts map[string]pki.CertificatePKI) error {
	var errgrp errgroup.Group
	hostsQueue := util.GetObjectQueue(c.EtcdHosts)
	restoreCerts := map[string]pki.CertificatePKI{}
	for _, n := range []string{pki.CACertName, pki.KubeNodeCertName, pki.KubeNodeCertName} {
		restoreCerts[n] = clusterCerts[n]
	}
	for w := 0; w < WorkerThreads; w++ {
		errgrp.Go(func() error {
			var errList []error
			for host := range hostsQueue {
				err := pki.DeployCertificatesOnPlaneHost(ctx, host.(*hosts.Host), c.RancherKubernetesEngineConfig, restoreCerts, c.SystemImages.CertDownloader, c.PrivateRegistriesMap, false)
				if err != nil {
					errList = append(errList, err)
				}
			}
			return util.ErrList(errList)
		})
	}
	if err := errgrp.Wait(); err != nil {
		return err
	}
	return nil
}

func (c *Cluster) PrepareBackup(ctx context.Context, snapshotPath string) error {
	// local backup case
	var backupReady bool
	var backupServer *hosts.Host
	backupImage := c.getBackupImage()
	var errors []error
	if c.Services.Etcd.BackupConfig == nil || // legacy rke local backup
		(c.Services.Etcd.BackupConfig != nil && c.Services.Etcd.BackupConfig.S3BackupConfig == nil) { // rancher local backup
		if c.Services.Etcd.BackupConfig == nil {
			log.Infof(ctx, "[etcd] No etcd snapshot configuration found, will use local as source")
		}
		if c.Services.Etcd.BackupConfig != nil && c.Services.Etcd.BackupConfig.S3BackupConfig == nil {
			log.Infof(ctx, "[etcd] etcd snapshot configuration found and no s3 backup configuration found, will use local as source")
		}
		// stop etcd on all etcd nodes, we need this because we start the backup server on the same port
		for _, host := range c.EtcdHosts {
			if err := docker.StopContainer(ctx, host.DClient, host.Address, services.EtcdContainerName); err != nil {
				log.Warnf(ctx, "failed to stop etcd container on host [%s]: %v", host.Address, err)
			}
			// start the download server, only one node should have it!
			if err := services.StartBackupServer(ctx, host, c.PrivateRegistriesMap, backupImage, snapshotPath); err != nil {
				log.Warnf(ctx, "failed to start backup server on host [%s]: %v", host.Address, err)
				errors = append(errors, err)
				continue
			}
			backupServer = host
			break
		}

		if backupServer == nil { //failed to start the backupServer, I will cleanup and exit
			for _, host := range c.EtcdHosts {
				if err := docker.StartContainer(ctx, host.DClient, host.Address, services.EtcdContainerName); err != nil {
					log.Warnf(ctx, "failed to start etcd container on host [%s]: %v", host.Address, err)
				}
			}
			return fmt.Errorf("failed to start backup server on all etcd nodes: %v", errors)
		}
		// start downloading the snapshot
		for _, host := range c.EtcdHosts {
			if host.Address == backupServer.Address { // we skip the backup server if it's there
				continue
			}
			if err := services.DownloadEtcdSnapshotFromBackupServer(ctx, host, c.PrivateRegistriesMap, backupImage, snapshotPath, backupServer); err != nil {
				return err
			}
		}
		// all good, let's remove the backup server container
		if err := docker.DoRemoveContainer(ctx, backupServer.DClient, services.EtcdServeBackupContainerName, backupServer.Address); err != nil {
			return err
		}
		backupReady = true
	}

	// s3 backup case
	if c.Services.Etcd.BackupConfig != nil &&
		c.Services.Etcd.BackupConfig.S3BackupConfig != nil {
		log.Infof(ctx, "[etcd] etcd s3 backup configuration found, will use s3 as source")
		for _, host := range c.EtcdHosts {
			if err := services.DownloadEtcdSnapshotFromS3(ctx, host, c.PrivateRegistriesMap, backupImage, snapshotPath, c.Services.Etcd); err != nil {
				return err
			}
		}
		backupReady = true
	}
	if !backupReady {
		return fmt.Errorf("failed to prepare backup for restore")
	}
	// this applies to all cases!
	if isEqual := c.etcdSnapshotChecksum(ctx, snapshotPath); !isEqual {
		return fmt.Errorf("etcd snapshots are not consistent")
	}
	return nil
}

func (c *Cluster) RestoreEtcdSnapshot(ctx context.Context, snapshotPath string) error {
	// Start restore process on all etcd hosts
	initCluster := services.GetEtcdInitialCluster(c.EtcdHosts)
	backupImage := c.getBackupImage()
	for _, host := range c.EtcdHosts {
		if err := services.RestoreEtcdSnapshot(ctx, host, c.PrivateRegistriesMap, c.SystemImages.Etcd, backupImage,
			snapshotPath, initCluster, c.Services.Etcd); err != nil {
			return fmt.Errorf("[etcd] Failed to restore etcd snapshot: %v", err)
		}
	}
	return nil
}

func (c *Cluster) RemoveEtcdSnapshot(ctx context.Context, snapshotName string) error {
	backupImage := c.getBackupImage()
	for _, host := range c.EtcdHosts {
		if err := services.RunEtcdSnapshotRemove(ctx, host, c.PrivateRegistriesMap, backupImage, snapshotName,
			false, c.Services.Etcd); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cluster) etcdSnapshotChecksum(ctx context.Context, snapshotPath string) bool {
	log.Infof(ctx, "[etcd] Checking if all snapshots are identical")
	etcdChecksums := []string{}
	backupImage := c.getBackupImage()

	for _, etcdHost := range c.EtcdHosts {
		checksum, err := services.GetEtcdSnapshotChecksum(ctx, etcdHost, c.PrivateRegistriesMap, backupImage, snapshotPath)
		if err != nil {
			return false
		}
		etcdChecksums = append(etcdChecksums, checksum)
		log.Infof(ctx, "[etcd] Checksum of etcd snapshot on host [%s] is [%s]", etcdHost.Address, checksum)
	}
	hostChecksum := etcdChecksums[0]
	for _, checksum := range etcdChecksums {
		if checksum != hostChecksum {
			return false
		}
	}
	return true
}

func (c *Cluster) getBackupImage() string {
	rkeToolsImage, err := util.GetDefaultRKETools(c.SystemImages.Alpine)
	if err != nil {
		logrus.Errorf("[etcd] error getting backup image %v", err)
		return ""
	}
	logrus.Debugf("[etcd] Image used for etcd snapshot is: [%s]", rkeToolsImage)
	return rkeToolsImage
}
