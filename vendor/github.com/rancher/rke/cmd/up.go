package cmd

import (
	"fmt"

	"github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/pki"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/client-go/util/cert"
)

func UpCommand() cli.Command {
	upFlags := []cli.Flag{
		cli.StringFlag{
			Name:   "config",
			Usage:  "Specify an alternate cluster YAML file",
			Value:  cluster.DefaultClusterConfig,
			EnvVar: "RKE_CONFIG",
		},
	}
	return cli.Command{
		Name:   "up",
		Usage:  "Bring the cluster up",
		Action: clusterUpFromCli,
		Flags:  upFlags,
	}
}

func ClusterUp(clusterFile string, customDialer hosts.Dialer) (string, string, string, string, error) {
	logrus.Infof("Building Kubernetes cluster")
	var APIURL, caCrt, clientCert, clientKey string
	kubeCluster, err := cluster.ParseConfig(clusterFile, customDialer)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, err
	}

	err = kubeCluster.TunnelHosts()
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, err
	}

	currentCluster, err := kubeCluster.GetClusterState()
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, err
	}

	if err := cluster.CheckEtcdHostsChanged(kubeCluster, currentCluster); err != nil {
		return APIURL, caCrt, clientCert, clientKey, err
	}

	err = cluster.SetUpAuthentication(kubeCluster, currentCluster)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, err
	}

	err = cluster.ReconcileCluster(kubeCluster, currentCluster)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, err
	}

	err = kubeCluster.SetUpHosts()
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, err
	}

	err = kubeCluster.DeployClusterPlanes()
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, err
	}

	err = kubeCluster.SaveClusterState(clusterFile)
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, err
	}

	err = kubeCluster.DeployNetworkPlugin()
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, err
	}

	err = kubeCluster.DeployK8sAddOns()
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, err
	}

	err = kubeCluster.DeployUserAddOns()
	if err != nil {
		return APIURL, caCrt, clientCert, clientKey, err
	}

	APIURL = fmt.Sprintf("https://" + kubeCluster.ControlPlaneHosts[0].Address + ":6443")
	caCrt = string(cert.EncodeCertPEM(kubeCluster.Certificates[pki.CACertName].Certificate))
	clientCert = string(cert.EncodeCertPEM(kubeCluster.Certificates[pki.KubeAdminCommonName].Certificate))
	clientKey = string(cert.EncodePrivateKeyPEM(kubeCluster.Certificates[pki.KubeAdminCommonName].Key))

	logrus.Infof("Finished building Kubernetes cluster successfully")
	return APIURL, caCrt, clientCert, clientKey, nil
}

func clusterUpFromCli(ctx *cli.Context) error {
	clusterFile, err := resolveClusterFile(ctx)
	if err != nil {
		return fmt.Errorf("Failed to resolve cluster file: %v", err)
	}
	_, _, _, _, err = ClusterUp(clusterFile, nil)
	return err
}
