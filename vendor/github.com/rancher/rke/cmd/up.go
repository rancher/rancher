package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/dind"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/log"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/urfave/cli"
	"k8s.io/client-go/util/cert"
)

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
		cli.StringFlag{
			Name:  "dind-storage-driver",
			Usage: "Storage driver for the docker in docker containers (experimental)",
		},
		cli.StringFlag{
			Name:  "dind-dns-server",
			Usage: "DNS resolver to be used by docker in docker container. Useful if host is running systemd-resovld",
			Value: "8.8.8.8",
		},
		cli.BoolFlag{
			Name:  "update-only",
			Usage: "Skip idempotent deployment of control and etcd plane",
		},
		cli.BoolFlag{
			Name:  "disable-port-check",
			Usage: "Disable port check validation between nodes",
		},
		cli.BoolFlag{
			Name:  "init",
			Usage: "Initiate RKE cluster",
		},
		cli.StringFlag{
			Name:  "cert-dir",
			Usage: "Specify a certificate dir path",
		},
		cli.BoolFlag{
			Name:  "custom-certs",
			Usage: "Use custom certificates from a cert dir",
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

func ClusterUp(ctx context.Context, dialersOptions hosts.DialersOptions, flags cluster.ExternalFlags, data map[string]interface{}) (string, string, string, string, map[string]pki.CertificatePKI, error) {
	var APIURL, caCrt, clientCert, clientKey string

	clusterState, err := cluster.ReadStateFile(ctx, cluster.GetStateFilePath(flags.ClusterFilePath, flags.ConfigDir))
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}
	kubeCluster, err := cluster.InitClusterObject(ctx, clusterState.DesiredState.RancherKubernetesEngineConfig.DeepCopy(), flags)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}
	svcOptions, _ := data["k8s-service-options"].(*v3.KubernetesServicesOptions)
	// check if rotate certificates is triggered
	if kubeCluster.RancherKubernetesEngineConfig.RotateCertificates != nil {
		return rebuildClusterWithRotatedCertificates(ctx, dialersOptions, flags, svcOptions)
	}

	log.Infof(ctx, "Building Kubernetes cluster")
	err = kubeCluster.SetupDialers(ctx, dialersOptions)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}
	err = kubeCluster.TunnelHosts(ctx, flags)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	currentCluster, err := kubeCluster.GetClusterState(ctx, clusterState)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	if !flags.DisablePortCheck {
		if err = kubeCluster.CheckClusterPorts(ctx, currentCluster); err != nil {
			return APIURL, caCrt, clientCert, clientKey, nil, err
		}
	}

	err = cluster.SetUpAuthentication(ctx, kubeCluster, currentCluster, clusterState)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}
	if len(kubeCluster.ControlPlaneHosts) > 0 {
		APIURL = fmt.Sprintf("https://" + kubeCluster.ControlPlaneHosts[0].Address + ":6443")
	}
	clientCert = string(cert.EncodeCertPEM(kubeCluster.Certificates[pki.KubeAdminCertName].Certificate))
	clientKey = string(cert.EncodePrivateKeyPEM(kubeCluster.Certificates[pki.KubeAdminCertName].Key))
	caCrt = string(cert.EncodeCertPEM(kubeCluster.Certificates[pki.CACertName].Certificate))

	// moved deploying certs before reconcile to remove all unneeded certs generation from reconcile
	err = kubeCluster.SetUpHosts(ctx, flags)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = cluster.ReconcileCluster(ctx, kubeCluster, currentCluster, flags, svcOptions)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}
	// update APIURL after reconcile
	if len(kubeCluster.ControlPlaneHosts) > 0 {
		APIURL = fmt.Sprintf("https://" + kubeCluster.ControlPlaneHosts[0].Address + ":6443")
	}

	if err := kubeCluster.PrePullK8sImages(ctx); err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.DeployControlPlane(ctx, svcOptions)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	// Apply Authz configuration after deploying controlplane
	err = cluster.ApplyAuthzResources(ctx, kubeCluster.RancherKubernetesEngineConfig, flags, dialersOptions)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.UpdateClusterCurrentState(ctx, clusterState)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = cluster.SaveFullStateToKubernetes(ctx, kubeCluster, clusterState)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.DeployWorkerPlane(ctx, svcOptions)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	if err = kubeCluster.CleanDeadLogs(ctx); err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = kubeCluster.SyncLabelsAndTaints(ctx, currentCluster)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

	err = cluster.ConfigureCluster(ctx, kubeCluster.RancherKubernetesEngineConfig, kubeCluster.Certificates, flags, dialersOptions, data, false)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, nil, err
	}

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
	// setting up the flags
	flags := cluster.GetExternalFlags(false, updateOnly, disablePortCheck, "", filePath)
	// Custom certificates and certificate dir flags
	flags.CertificateDir = ctx.String("cert-dir")
	flags.CustomCerts = ctx.Bool("custom-certs")
	if ctx.Bool("init") {
		return ClusterInit(context.Background(), rkeConfig, hosts.DialersOptions{}, flags)
	}
	if err := ClusterInit(context.Background(), rkeConfig, hosts.DialersOptions{}, flags); err != nil {
		return err
	}

	_, _, _, _, _, err = ClusterUp(context.Background(), hosts.DialersOptions{}, flags, map[string]interface{}{})
	return err
}

func clusterUpLocal(ctx *cli.Context) error {
	var rkeConfig *v3.RancherKubernetesEngineConfig
	clusterFile, filePath, err := resolveClusterFile(ctx)
	if err != nil {
		log.Infof(context.Background(), "Failed to resolve cluster file, using default cluster instead")
		rkeConfig = cluster.GetLocalRKEConfig()
	} else {
		rkeConfig, err = cluster.ParseConfig(clusterFile)
		if err != nil {
			return fmt.Errorf("Failed to parse cluster file: %v", err)
		}
		rkeConfig.Nodes = []v3.RKEConfigNode{*cluster.GetLocalRKENodeConfig()}
	}

	rkeConfig.IgnoreDockerVersion = ctx.Bool("ignore-docker-version")

	// setting up the dialers
	dialers := hosts.GetDialerOptions(nil, hosts.LocalHealthcheckFactory, nil)
	// setting up the flags
	flags := cluster.GetExternalFlags(true, false, false, "", filePath)

	if ctx.Bool("init") {
		return ClusterInit(context.Background(), rkeConfig, dialers, flags)
	}
	if err := ClusterInit(context.Background(), rkeConfig, dialers, flags); err != nil {
		return err
	}
	_, _, _, _, _, err = ClusterUp(context.Background(), dialers, flags, map[string]interface{}{})
	return err
}

func clusterUpDind(ctx *cli.Context) error {
	// get dind config
	rkeConfig, disablePortCheck, dindStorageDriver, filePath, dindDNS, err := getDindConfig(ctx)
	if err != nil {
		return err
	}
	// setup dind environment
	if err = createDINDEnv(context.Background(), rkeConfig, dindStorageDriver, dindDNS); err != nil {
		return err
	}

	// setting up the dialers
	dialers := hosts.GetDialerOptions(hosts.DindConnFactory, hosts.DindHealthcheckConnFactory, nil)
	// setting up flags
	flags := cluster.GetExternalFlags(false, false, disablePortCheck, "", filePath)
	flags.DinD = true

	if ctx.Bool("init") {
		return ClusterInit(context.Background(), rkeConfig, dialers, flags)
	}
	if err := ClusterInit(context.Background(), rkeConfig, dialers, flags); err != nil {
		return err
	}
	// start cluster
	_, _, _, _, _, err = ClusterUp(context.Background(), dialers, flags, map[string]interface{}{})
	return err
}

func getDindConfig(ctx *cli.Context) (*v3.RancherKubernetesEngineConfig, bool, string, string, string, error) {
	disablePortCheck := ctx.Bool("disable-port-check")
	dindStorageDriver := ctx.String("dind-storage-driver")
	dindDNS := ctx.String("dind-dns-server")

	clusterFile, filePath, err := resolveClusterFile(ctx)
	if err != nil {
		return nil, disablePortCheck, "", "", "", fmt.Errorf("Failed to resolve cluster file: %v", err)
	}

	rkeConfig, err := cluster.ParseConfig(clusterFile)
	if err != nil {
		return nil, disablePortCheck, "", "", "", fmt.Errorf("Failed to parse cluster file: %v", err)
	}

	rkeConfig, err = setOptionsFromCLI(ctx, rkeConfig)
	if err != nil {
		return nil, disablePortCheck, "", "", "", err
	}
	// Setting conntrack max for kubeproxy to 0
	if rkeConfig.Services.Kubeproxy.ExtraArgs == nil {
		rkeConfig.Services.Kubeproxy.ExtraArgs = make(map[string]string)
	}
	rkeConfig.Services.Kubeproxy.ExtraArgs["conntrack-max-per-core"] = "0"

	return rkeConfig, disablePortCheck, dindStorageDriver, filePath, dindDNS, nil
}

func createDINDEnv(ctx context.Context, rkeConfig *v3.RancherKubernetesEngineConfig, dindStorageDriver, dindDNS string) error {
	for i := range rkeConfig.Nodes {
		address, err := dind.StartUpDindContainer(ctx, rkeConfig.Nodes[i].Address, dind.DINDNetwork, dindStorageDriver, dindDNS)
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
