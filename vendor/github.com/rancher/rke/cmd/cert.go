package cmd

import (
	"context"
	"fmt"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
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
	clusterFilePath = filePath

	rkeConfig, err := cluster.ParseConfig(clusterFile)
	if err != nil {
		return fmt.Errorf("Failed to parse cluster file: %v", err)
	}
	rkeConfig, err = setOptionsFromCLI(ctx, rkeConfig)
	if err != nil {
		return err
	}

	return RotateRKECertificates(context.Background(), rkeConfig, nil, nil, nil, false, "", k8sComponent, rotateCACert)
}

func showRKECertificatesFromCli(ctx *cli.Context) error {
	return nil
}

func RotateRKECertificates(
	ctx context.Context,
	rkeConfig *v3.RancherKubernetesEngineConfig,
	dockerDialerFactory, localConnDialerFactory hosts.DialerFactory,
	k8sWrapTransport k8s.WrapTransport,
	local bool, configDir string, components []string, rotateCACerts bool) error {

	log.Infof(ctx, "Rotating Kubernetes cluster certificates")
	kubeCluster, err := cluster.ParseCluster(ctx, rkeConfig, clusterFilePath, configDir, dockerDialerFactory, localConnDialerFactory, k8sWrapTransport)
	if err != nil {
		return err
	}

	if err := kubeCluster.TunnelHosts(ctx, local); err != nil {
		return err
	}

	currentCluster, err := kubeCluster.GetClusterState(ctx)
	if err != nil {
		return err
	}

	if err := cluster.SetUpAuthentication(ctx, kubeCluster, currentCluster); err != nil {
		return err
	}

	if err := cluster.RotateRKECertificates(ctx, kubeCluster, clusterFilePath, configDir, components, rotateCACerts); err != nil {
		return err
	}

	if err := kubeCluster.SetUpHosts(ctx, true); err != nil {
		return err
	}
	// Restarting Kubernetes components
	servicesMap := make(map[string]bool)
	for _, component := range components {
		servicesMap[component] = true
	}

	if len(components) == 0 || rotateCACerts || servicesMap[services.EtcdContainerName] {
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

	if err := kubeCluster.SaveClusterState(ctx, &kubeCluster.RancherKubernetesEngineConfig); err != nil {
		return err
	}

	if rotateCACerts {
		return cluster.RestartClusterPods(ctx, kubeCluster)
	}
	return nil
}
