package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/util"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var commonFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "ssh-agent-auth",
		Usage: "Use SSH Agent Auth defined by SSH_AUTH_SOCK",
	},
	cli.BoolFlag{
		Name:  "ignore-docker-version",
		Usage: "Disable Docker version check",
	},
}

func resolveClusterFile(ctx *cli.Context) (string, string, error) {
	clusterFile := ctx.String("config")
	fp, err := filepath.Abs(clusterFile)
	if err != nil {
		return "", "", fmt.Errorf("failed to lookup current directory name: %v", err)
	}
	file, err := os.Open(fp)
	if err != nil {
		return "", "", fmt.Errorf("can not find cluster configuration file: %v", err)
	}
	defer file.Close()
	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return "", "", fmt.Errorf("failed to read file: %v", err)
	}
	clusterFileBuff := string(buf)
	return clusterFileBuff, clusterFile, nil
}

func setOptionsFromCLI(c *cli.Context, rkeConfig *v3.RancherKubernetesEngineConfig) (*v3.RancherKubernetesEngineConfig, error) {
	// If true... override the file.. else let file value go through
	if c.Bool("ssh-agent-auth") {
		rkeConfig.SSHAgentAuth = c.Bool("ssh-agent-auth")
	}

	if c.Bool("ignore-docker-version") {
		rkeConfig.IgnoreDockerVersion = c.Bool("ignore-docker-version")
	}

	if c.Bool("s3") {
		if rkeConfig.Services.Etcd.BackupConfig == nil {
			rkeConfig.Services.Etcd.BackupConfig = &v3.BackupConfig{}
		}
		rkeConfig.Services.Etcd.BackupConfig.S3BackupConfig = setS3OptionsFromCLI(c)
	}
	return rkeConfig, nil
}

func ClusterInit(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, dialersOptions hosts.DialersOptions, flags cluster.ExternalFlags) error {
	log.Infof(ctx, "Initiating Kubernetes cluster")
	var fullState *cluster.FullState
	stateFilePath := cluster.GetStateFilePath(flags.ClusterFilePath, flags.ConfigDir)
	if len(flags.CertificateDir) == 0 {
		flags.CertificateDir = cluster.GetCertificateDirPath(flags.ClusterFilePath, flags.ConfigDir)
	}
	rkeFullState, _ := cluster.ReadStateFile(ctx, stateFilePath)
	kubeCluster, err := cluster.InitClusterObject(ctx, rkeConfig, flags, rkeFullState.DesiredState.EncryptionConfig)
	if err != nil {
		return err
	}

	if err := kubeCluster.SetupDialers(ctx, dialersOptions); err != nil {
		return err
	}

	err = checkLegacyCluster(ctx, kubeCluster, rkeFullState, flags)
	if err != nil {
		if strings.Contains(err.Error(), "aborting upgrade") {
			return err
		}
		log.Warnf(ctx, "[state] can't fetch legacy cluster state from Kubernetes: %v", err)
	}

	// check if certificate rotate or normal init
	if kubeCluster.RancherKubernetesEngineConfig.RotateCertificates != nil {
		fullState, err = rotateRKECertificates(ctx, kubeCluster, flags, rkeFullState)
	} else {
		fullState, err = cluster.RebuildState(ctx, kubeCluster, rkeFullState, flags)
	}
	if err != nil {
		return err
	}

	if fullState.DesiredState.EncryptionConfig != "" {
		kubeCluster.EncryptionConfig.EncryptionProviderFile = fullState.DesiredState.EncryptionConfig
	}

	rkeState := cluster.FullState{
		DesiredState: fullState.DesiredState,
		CurrentState: fullState.CurrentState,
	}
	return rkeState.WriteStateFile(ctx, stateFilePath)
}

func setS3OptionsFromCLI(c *cli.Context) *v3.S3BackupConfig {
	endpoint := c.String("s3-endpoint")
	bucketName := c.String("bucket-name")
	region := c.String("region")
	accessKey := c.String("access-key")
	secretKey := c.String("secret-key")
	endpointCA := c.String("s3-endpoint-ca")
	folder := c.String("folder")
	var s3BackupBackend = &v3.S3BackupConfig{}
	if len(endpoint) != 0 {
		s3BackupBackend.Endpoint = endpoint
	}
	if len(bucketName) != 0 {
		s3BackupBackend.BucketName = bucketName
	}
	if len(region) != 0 {
		s3BackupBackend.Region = region
	}
	if len(accessKey) != 0 {
		s3BackupBackend.AccessKey = accessKey
	}
	if len(secretKey) != 0 {
		s3BackupBackend.SecretKey = secretKey
	}
	if len(endpointCA) != 0 {
		caStr, err := pki.ReadCertToStr(endpointCA)
		if err != nil {
			logrus.Warnf("Failed to read s3-endpoint-ca [%s]: %v", endpointCA, err)
		} else {
			s3BackupBackend.CustomCA = caStr
		}
	}
	if len(folder) != 0 {
		s3BackupBackend.Folder = folder
	}
	return s3BackupBackend
}

func checkLegacyCluster(ctx context.Context, kubeCluster *cluster.Cluster, fullState *cluster.FullState, flags cluster.ExternalFlags) error {
	stateFileExists, err := util.IsFileExists(kubeCluster.StateFilePath)
	if err != nil {
		return err
	}
	if stateFileExists {
		logrus.Debug("[state] previous state found, this is not a legacy cluster")
		return nil
	}
	logrus.Debug("[state] previous state not found, possible legacy cluster")
	return fetchAndUpdateStateFromLegacyCluster(ctx, kubeCluster, fullState, flags)
}

func fetchAndUpdateStateFromLegacyCluster(ctx context.Context, kubeCluster *cluster.Cluster, fullState *cluster.FullState, flags cluster.ExternalFlags) error {
	kubeConfigExists, err := util.IsFileExists(kubeCluster.LocalKubeConfigPath)
	if err != nil {
		return err
	}
	if !kubeConfigExists {
		// if kubeconfig doesn't exist and its a legacy cluster then error out
		if err := kubeCluster.TunnelHosts(ctx, flags); err != nil {
			return err
		}
		if recoveredCluster := cluster.GetStateFromNodes(ctx, kubeCluster); recoveredCluster != nil {
			return fmt.Errorf("This is a legacy cluster with no kube config, aborting upgrade. Please re-run rke up with rke 0.1.x to retrieve correct state")
		}
		return nil
	}
	// We have a kubeconfig and no current state. This is a legacy cluster or a new cluster with old kubeconfig
	// let's try to upgrade
	log.Infof(ctx, "[state] Possible legacy cluster detected, trying to upgrade")
	if err := cluster.RebuildKubeconfig(ctx, kubeCluster); err != nil {
		return err
	}
	recoveredCluster, err := cluster.GetStateFromKubernetes(ctx, kubeCluster)
	if err != nil {
		log.Warnf(ctx, "Failed to fetch state from kubernetes: %v", err)
		// try to fetch state from nodes
		err = kubeCluster.TunnelHosts(ctx, flags)
		if err != nil {
			return err
		}
		// fetching state/certs from nodes should be removed in rke 0.3.0
		log.Infof(ctx, "[state] Fetching cluster state from Nodes")
		recoveredCluster = cluster.GetStateFromNodes(ctx, kubeCluster)
	}
	// if we found a recovered cluster, we will need override the current state
	recoveredCerts, err := cluster.GetClusterCertsFromKubernetes(ctx, kubeCluster)
	if err != nil {
		log.Warnf(ctx, "Failed to fetch certs from kubernetes: %v", err)
		// try to fetch certs from nodes
		recoveredCerts, err = cluster.GetClusterCertsFromNodes(ctx, kubeCluster)
		if err != nil {
			return fmt.Errorf("Failed to fetch cluster certs from nodes, aborting upgrade: %v", err)
		}
	}
	fullState.CurrentState.RancherKubernetesEngineConfig = kubeCluster.RancherKubernetesEngineConfig.DeepCopy()
	if recoveredCluster != nil {
		fullState.CurrentState.RancherKubernetesEngineConfig = recoveredCluster.RancherKubernetesEngineConfig.DeepCopy()
	}

	fullState.CurrentState.CertificatesBundle = recoveredCerts

	// we don't want to regenerate certificates
	fullState.DesiredState.CertificatesBundle = recoveredCerts
	return fullState.WriteStateFile(ctx, kubeCluster.StateFilePath)
}
