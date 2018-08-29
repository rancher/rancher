package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/dind"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/k8s"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/urfave/cli"
	"k8s.io/client-go/util/cert"
)

var clusterFilePath string

const DINDWaitTime = 3

func UpCommand() cli.Command {
	upFlags := []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Usage:  "Specify an alternate cluster YAML file",
			Value:  pki.ClusterConfig,
			EnvVar: "RKE_CONFIG",
		},
		cli.BoolFlag{
			Name:  "local",
			Usage: "Deploy Kubernetes cluster locally",
		},
		cli.BoolFlag{
			Name:  "dind",
			Usage: "Deploy Kubernetes cluster in docker containers (experimental)",
		},
		cli.BoolFlag{
			Name:  "update-only",
			Usage: "Skip idempotent deployment of control and etcd plane",
		},
		cli.BoolFlag{
			Name:  "disable-port-check",
			Usage: "Disable port check validation between nodes",
		},
	}

	upFlags = append(upFlags, commonFlags...)

	return cli.Command{
		Name:   "up",
		Usage:  "Bring the cluster up",
		Action: clusterUpFromCli,
		Flags:  upFlags,
	}
}

func ClusterUp(
	ctx context.Context,
	rkeConfig *v3.RancherKubernetesEngineConfig,
	dockerDialerFactory, localConnDialerFactory hosts.DialerFactory,
	k8sWrapTransport k8s.WrapTransport,
	local bool, configDir string, updateOnly, disablePortCheck bool) (string, string, string, string, map[string]pki.CertificatePKI, error) {

	log.Infof(ctx, "Building Kubernetes cluster")
	var APIURL, caCrt, clientCert, clientKey string
	kubeCluster, err := cluster.ParseCluster(ctx, rkeConfig, clusterFilePath, configDir, dockerDialerFactory, localConnDialerFactory, k8sWrapTransport)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.TunnelHosts(ctx, local)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	currentCluster, err := kubeCluster.GetClusterState(ctx)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}
	if !disablePortCheck {
		if err = kubeCluster.CheckClusterPorts(ctx, currentCluster); err != nil {
			return APIURL, caCrt, clientCert, clientKey, nil, err
		}
	}

	err = cluster.SetUpAuthentication(ctx, kubeCluster, currentCluster)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = cluster.ReconcileCluster(ctx, kubeCluster, currentCluster, updateOnly)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.SetUpHosts(ctx)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	if err := kubeCluster.PrePullK8sImages(ctx); err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.DeployControlPlane(ctx)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	// Apply Authz configuration after deploying controlplane
	err = cluster.ApplyAuthzResources(ctx, kubeCluster.RancherKubernetesEngineConfig, clusterFilePath, configDir, k8sWrapTransport)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.SaveClusterState(ctx, &kubeCluster.RancherKubernetesEngineConfig)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.DeployWorkerPlane(ctx)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	if err = kubeCluster.CleanDeadLogs(ctx); err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.SyncLabelsAndTaints(ctx)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = cluster.ConfigureCluster(ctx, kubeCluster.RancherKubernetesEngineConfig, kubeCluster.Certificates, clusterFilePath, configDir, k8sWrapTransport, false)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}
	if len(kubeCluster.ControlPlaneHosts) > 0 {
		APIURL = fmt.Sprintf("https://" + kubeCluster.ControlPlaneHosts[0].Address + ":6443")
		clientCert = string(cert.EncodeCertPEM(kubeCluster.Certificates[pki.KubeAdminCertName].Certificate))
		clientKey = string(cert.EncodePrivateKeyPEM(kubeCluster.Certificates[pki.KubeAdminCertName].Key))
	}
	caCrt = string(cert.EncodeCertPEM(kubeCluster.Certificates[pki.CACertName].Certificate))

	if err := checkAllIncluded(kubeCluster); err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	log.Infof(ctx, "Finished building Kubernetes cluster successfully")
	return APIURL, caCrt, clientCert, clientKey, kubeCluster.Certificates, nil
}

func checkAllIncluded(cluster *cluster.Cluster) error {
	if len(cluster.InactiveHosts) == 0 {
		return nil
	}

	var names []string
	for _, host := range cluster.InactiveHosts {
		names = append(names, host.Address)
	}

	return fmt.Errorf("Provisioning incomplete, host(s) [%s] skipped because they could not be contacted", strings.Join(names, ","))
}

func clusterUpFromCli(ctx *cli.Context) error {
	if ctx.Bool("local") {
		return clusterUpLocal(ctx)
	}
	if ctx.Bool("dind") {
		return clusterUpDind(ctx)
	}
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
	updateOnly := ctx.Bool("update-only")
	disablePortCheck := ctx.Bool("disable-port-check")

	_, _, _, _, _, err = ClusterUp(context.Background(), rkeConfig, nil, nil, nil, false, "", updateOnly, disablePortCheck)
	return err
}

func clusterUpLocal(ctx *cli.Context) error {
	var rkeConfig *v3.RancherKubernetesEngineConfig
	clusterFile, filePath, err := resolveClusterFile(ctx)
	if err != nil {
		log.Infof(context.Background(), "Failed to resolve cluster file, using default cluster instead")
		rkeConfig = cluster.GetLocalRKEConfig()
	} else {
		clusterFilePath = filePath
		rkeConfig, err = cluster.ParseConfig(clusterFile)
		if err != nil {
			return fmt.Errorf("Failed to parse cluster file: %v", err)
		}
		rkeConfig.Nodes = []v3.RKEConfigNode{*cluster.GetLocalRKENodeConfig()}
	}

	rkeConfig.IgnoreDockerVersion = ctx.Bool("ignore-docker-version")

	_, _, _, _, _, err = ClusterUp(context.Background(), rkeConfig, nil, hosts.LocalHealthcheckFactory, nil, true, "", false, false)
	return err
}

func clusterUpDind(ctx *cli.Context) error {
	// get dind config
	rkeConfig, disablePortCheck, err := getDindConfig(ctx)
	if err != nil {
		return err
	}
	// setup dind environment
	if err = createDINDEnv(context.Background(), rkeConfig); err != nil {
		return err
	}
	// start cluster
	_, _, _, _, _, err = ClusterUp(context.Background(), rkeConfig, hosts.DindConnFactory, hosts.DindHealthcheckConnFactory, nil, false, "", false, disablePortCheck)
	return err
}

func getDindConfig(ctx *cli.Context) (*v3.RancherKubernetesEngineConfig, bool, error) {
	disablePortCheck := ctx.Bool("disable-port-check")
	clusterFile, filePath, err := resolveClusterFile(ctx)
	if err != nil {
		return nil, disablePortCheck, fmt.Errorf("Failed to resolve cluster file: %v", err)
	}
	clusterFilePath = filePath

	rkeConfig, err := cluster.ParseConfig(clusterFile)
	if err != nil {
		return nil, disablePortCheck, fmt.Errorf("Failed to parse cluster file: %v", err)
	}

	rkeConfig, err = setOptionsFromCLI(ctx, rkeConfig)
	if err != nil {
		return nil, disablePortCheck, err
	}
	// Setting conntrack max for kubeproxy to 0
	if rkeConfig.Services.Kubeproxy.ExtraArgs == nil {
		rkeConfig.Services.Kubeproxy.ExtraArgs = make(map[string]string)
	}
	rkeConfig.Services.Kubeproxy.ExtraArgs["conntrack-max-per-core"] = "0"

	return rkeConfig, disablePortCheck, nil
}

func createDINDEnv(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig) error {
	for i := range rkeConfig.Nodes {
		address, err := dind.StartUpDindContainer(ctx, rkeConfig.Nodes[i].Address, dind.DINDNetwork)
		if err != nil {
			return err
		}
		if rkeConfig.Nodes[i].HostnameOverride == "" {
			rkeConfig.Nodes[i].HostnameOverride = rkeConfig.Nodes[i].Address
		}
		rkeConfig.Nodes[i].Address = address
	}
	time.Sleep(DINDWaitTime * time.Second)
	return nil
}
