package cmd

import (
	"context"
	"fmt"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/urfave/cli"
)

func CertificateCommand() cli.Command {
	return cli.Command{
		Name:  "cert",
		Usage: "Certificates management for RKE cluster",
		Subcommands: cli.Commands{
			cli.Command{
				Name:   "rotate",
				Usage:  "Rotate RKE cluster certificates",
				Action: rotateRKECertificatesFromCli,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:   "config",
						Usage:  "Specify an alternate cluster YAML file",
						Value:  pki.ClusterConfig,
						EnvVar: "RKE_CONFIG",
					},
					cli.StringSliceFlag{
						Name: "service",
						Usage: fmt.Sprintf("Specify a k8s service to rotate certs, (allowed values: %s, %s, %s, %s, %s, %s)",
							services.KubeAPIContainerName,
							services.KubeControllerContainerName,
							services.SchedulerContainerName,
							services.KubeletContainerName,
							services.KubeproxyContainerName,
							services.EtcdContainerName,
						),
					},
					cli.BoolFlag{
						Name:  "rotate-ca",
						Usage: "Rotate all certificates including CA certs",
					},
				},
			},
		},
	}
}

func rotateRKECertificatesFromCli(ctx *cli.Context) error {
	k8sComponent := ctx.StringSlice("service")
	rotateCACert := ctx.Bool("rotate-ca")
	clusterFile, filePath, err := resolveClusterFile(ctx)
	if err != nil {
		return fmt.Errorf("Failed to resolve cluster file: %v", err)
	}

	rkeConfig, err := cluster.ParseConfig(clusterFile)
	if err != nil {
		return fmt.Errorf("Failed to parse cluster file: %v", err)
	}
	rkeConfig, err = setOptionsFromCLI(ctx, rkeConfig)
	if err != nil {
		return err
	}
	// setting up the flags
	externalFlags := cluster.GetExternalFlags(false, false, false, "", filePath)
	rotateFlags := cluster.GetRotateCertsFlags(rotateCACert, k8sComponent)

	if err := RotateRKECertificates(context.Background(), rkeConfig, hosts.DialersOptions{}, externalFlags, rotateFlags); err != nil {
		return err
	}
	return RebuildClusterWithRotatedCertificates(context.Background(), rkeConfig, hosts.DialersOptions{}, externalFlags, rotateFlags)
}

func showRKECertificatesFromCli(ctx *cli.Context) error {
	return nil
}

func RebuildClusterWithRotatedCertificates(ctx context.Context,
	rkeConfig *v3.RancherKubernetesEngineConfig,
	dialersOptions hosts.DialersOptions,
	flags cluster.ExternalFlags,
	rotateFlags cluster.RotateCertificatesFlags) error {

	log.Infof(ctx, "Rebuilding Kubernetes cluster with rotated certificates")
	clusterState, err := cluster.ReadStateFile(ctx, cluster.GetStateFilePath(flags.ClusterFilePath, flags.ConfigDir))
	if err != nil {
		return err
	}

	kubeCluster, err := cluster.InitClusterObject(ctx, rkeConfig, flags)
	if err != nil {
		return err
	}
	if err := kubeCluster.SetupDialers(ctx, dialersOptions); err != nil {
		return err
	}

	if err := kubeCluster.TunnelHosts(ctx, flags); err != nil {
		return err
	}

	if err := cluster.SetUpAuthentication(ctx, kubeCluster, nil, clusterState); err != nil {
		return err
	}

	if err := kubeCluster.SetUpHosts(ctx, true); err != nil {
		return err
	}
	// Save new State
	if err := kubeCluster.UpdateClusterCurrentState(ctx, clusterState); err != nil {
		return err
	}

	// Restarting Kubernetes components
	servicesMap := make(map[string]bool)
	for _, component := range rotateFlags.RotateComponents {
		servicesMap[component] = true
	}

	if len(rotateFlags.RotateComponents) == 0 || rotateFlags.RotateCACerts || servicesMap[services.EtcdContainerName] {
		if err := services.RestartEtcdPlane(ctx, kubeCluster.EtcdHosts); err != nil {
			return err
		}
	}

	if err := services.RestartControlPlane(ctx, kubeCluster.ControlPlaneHosts); err != nil {
		return err
	}

	allHosts := hosts.GetUniqueHostList(kubeCluster.EtcdHosts, kubeCluster.ControlPlaneHosts, kubeCluster.WorkerHosts)
	if err := services.RestartWorkerPlane(ctx, allHosts); err != nil {
		return err
	}

	if rotateFlags.RotateCACerts {
		return cluster.RestartClusterPods(ctx, kubeCluster)
	}
	return nil
}

func RotateRKECertificates(ctx context.Context,
	rkeConfig *v3.RancherKubernetesEngineConfig,
	dialersOptions hosts.DialersOptions,
	flags cluster.ExternalFlags,
	rotateFlags cluster.RotateCertificatesFlags) error {
	log.Infof(ctx, "Rotating Kubernetes cluster certificates")
	stateFilePath := cluster.GetStateFilePath(flags.ClusterFilePath, flags.ConfigDir)
	clusterState, _ := cluster.ReadStateFile(ctx, stateFilePath)

	kubeCluster, err := cluster.InitClusterObject(ctx, rkeConfig, flags)
	if err != nil {
		return err
	}

	if err := kubeCluster.SetupDialers(ctx, dialersOptions); err != nil {
		return err
	}

	err = doUpgradeLegacyCluster(ctx, kubeCluster, clusterState)
	if err != nil {
		log.Warnf(ctx, "[state] can't fetch legacy cluster state from Kubernetes")
	}

	currentCluster, err := kubeCluster.GetClusterState(ctx, clusterState)
	if err != nil {
		return err
	}
	if currentCluster == nil {
		return fmt.Errorf("Failed to rotate certificates: can't find old certificates")
	}
	if err := cluster.RotateRKECertificates(ctx, currentCluster, flags, rotateFlags, clusterState); err != nil {
		return err
	}
	rkeState := cluster.FullState{
		DesiredState: clusterState.DesiredState,
		CurrentState: clusterState.CurrentState,
	}
	return rkeState.WriteStateFile(ctx, stateFilePath)
}
