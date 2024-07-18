package rke1

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/provisioning"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	"github.com/rancher/rancher/tests/framework/extensions/rke1"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	kubeConfigFile             = "kube_config_cluster.yml"
	RKEStandaloneConfigFileKey = "rkeStandalone"
)

type RKEStandaloneConfig struct {
	Version string `json:"version" yaml:"version"`
}

type ImportClusterTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	clustersConfig *provisioninginput.Config
	rkeVersion     string
}

func (i *ImportClusterTestSuite) TearDownSuite() {
	i.session.Cleanup()
}

func (i *ImportClusterTestSuite) SetupSuite() {
	testSession := session.NewSession()
	i.session = testSession

	i.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, i.clustersConfig)
	rkeCfg := &RKEStandaloneConfig{}
	config.LoadConfig(RKEStandaloneConfigFileKey, rkeCfg)
	i.rkeVersion = rkeCfg.Version

	client, err := rancher.NewClient("", testSession)
	require.NoError(i.T(), err)

	i.client = client

	i.clustersConfig.NodesAndRolesRKE1 = []nodepools.NodeRoles{provisioninginput.RKE1EtcdPool, provisioninginput.RKE1ControlPlanePool, provisioninginput.RKE1WorkerPool}
}

func (i *ImportClusterTestSuite) TestImportRKE1Cluster() {
	require.NotEmpty(i.T(), i.rkeVersion)

	subSession := i.session.NewSession()
	defer subSession.Cleanup()

	client, err := i.client.WithSession(subSession)
	require.NoError(i.T(), err)

	for _, k8sVersion := range i.clustersConfig.RKE1KubernetesVersions {
		for _, nodeProviderName := range i.clustersConfig.NodeProviders {
			var outb bytes.Buffer
			var stderrb bytes.Buffer

			clusteName := namegen.AppendRandomString("test-import")
			externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviderName)

			kubeCfg, err := CreateRKE1StandaloneCluster(client, externalNodeProvider, i.clustersConfig.NodesAndRolesRKE1, k8sVersion, i.rkeVersion)
			require.NoError(i.T(), err)

			err = ImportRKE1StandaloneCluster(client, clusteName, kubeCfg)
			require.NoError(i.T(), err)

			logrus.Info("running rke remove...")
			out := exec.Command("./rke_linux-amd64", "remove", "--force")
			out.Stdout = &outb
			out.Stderr = &stderrb
			err = out.Run()
			logrus.Infoln(outb.String())
			logrus.Infoln(stderrb.String())
			require.NoError(i.T(), err)

			err = os.Remove("rke_linux-amd64")
			assert.NoError(i.T(), err)

			err = os.Remove("cluster.yml")
			assert.NoError(i.T(), err)
		}
	}
}

func CreateRKE1StandaloneCluster(client *rancher.Client, externalNodeProvider provisioning.ExternalNodeProvider, nodesAndRoles []nodepools.NodeRoles, k8sVersion string, rkeVersion string) ([]byte, error) {
	rkeConfig := rke1.RKEConfig

	controlPlane := &nodes.Node{}
	etcd := &nodes.Node{}
	worker := &nodes.Node{}
	var outb bytes.Buffer
	var stderrb bytes.Buffer

	for _, nodeAndRole := range nodesAndRoles {
		logrus.Info("creating node...")
		if nodeAndRole.ControlPlane {
			cpNodes, err := externalNodeProvider.NodeCreationFunc(client, []string{"--controlplane"}, []int32{int32(nodeAndRole.Quantity)})
			if err != nil {
				return nil, err
			}
			for _, cpNode := range cpNodes {
				controlPlane = cpNode
				_, err := cpNode.ExecuteCommand(rke1.StartDockerCmd)
				if err != nil {
					return nil, err
				}
			}
		} else if nodeAndRole.Etcd {
			eNodes, err := externalNodeProvider.NodeCreationFunc(client, []string{"--etcd"}, []int32{int32(nodeAndRole.Quantity)})
			if err != nil {
				return nil, err
			}
			for _, eNode := range eNodes {
				etcd = eNode
				_, err := eNode.ExecuteCommand(rke1.StartDockerCmd)
				if err != nil {
					return nil, err
				}
			}
		} else {
			wNodes, err := externalNodeProvider.NodeCreationFunc(client, []string{"--worker"}, []int32{int32(nodeAndRole.Quantity)})
			if err != nil {
				return nil, err
			}
			for _, wNode := range wNodes {
				worker = wNode
				_, err := wNode.ExecuteCommand(rke1.StartDockerCmd)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	_, err := exec.Command("wget", "--timeout", "120", fmt.Sprintf(rke1.RKE_URL, rkeVersion)).Output()
	if err != nil {
		return nil, err
	}

	err = os.Chmod("rke_linux-amd64", 0777)
	if err != nil {
		return nil, err
	}

	f, err := os.Create("cluster.yml")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cmd := exec.Command("echo", fmt.Sprintf(rkeConfig, k8sVersion, controlPlane.PublicIPAddress, controlPlane.PrivateIPAddress, controlPlane.SSHUser,
		etcd.PublicIPAddress, etcd.PrivateIPAddress, etcd.SSHUser, worker.PublicIPAddress, worker.PrivateIPAddress, worker.SSHUser))

	cmd.Stdout = f

	if err := cmd.Run(); err != nil {
		return nil, err
	}
	logrus.Info("cluster.yml : ", cmd)

	logrus.Infof("running rke up...")

	out := exec.Command("./rke_linux-amd64", "up")
	out.Stdout = &outb
	out.Stderr = &stderrb
	err = out.Run()
	logrus.Info(outb.String())
	logrus.Info(stderrb.String())
	if err != nil {
		return nil, err
	}

	kubeConfig, err := os.ReadFile(kubeConfigFile)
	if err != nil {
		return nil, err
	}

	logrus.Info("kube-config : ", string(kubeConfig))
	return kubeConfig, err
}

func ImportRKE1StandaloneCluster(client *rancher.Client, clusterName string, kubeCfg []byte) error {
	var importTimeout = int64(60 * 20)
	var impCluster *apisV1.Cluster

	cluster := &apisV1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: "fleet-default",
		},
	}
	clusterObj, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).Create(cluster)
	if err != nil {
		return err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeCfg)
	if err != nil {
		return err
	}

	kubeProvisioningClient, err := client.GetKubeAPIProvisioningClient()
	if err != nil {
		return err
	}
	clusterWatch, err := kubeProvisioningClient.Clusters("fleet-default").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	err = wait.WatchWait(clusterWatch, func(event watch.Event) (bool, error) {
		cluster := event.Object.(*apisV1.Cluster)
		if cluster.Name == clusterName {
			impCluster, err = kubeProvisioningClient.Clusters("fleet-default").Get(context.TODO(), clusterName, metav1.GetOptions{})
			return true, err
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	logrus.Infof("Importing cluster...")
	err = clusters.ImportCluster(client, impCluster, restConfig)
	if err != nil {
		return err
	}

	clusterWatch, err = kubeProvisioningClient.Clusters("fleet-default").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &importTimeout,
	})
	if err != nil {
		return err
	}

	checkFunc := clusters.IsImportedClusterReady
	err = wait.WatchWait(clusterWatch, checkFunc)
	if err != nil {
		return err
	}

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return err
	}

	_, podErrors := pods.StatusPods(client, clusterID)
	if len(podErrors) > 0 {
		return fmt.Errorf("pods have error")
	}

	logrus.Info("deleting cluster...")
	err = client.Steve.SteveType(clusters.ProvisioningSteveResourceType).Delete(clusterObj)
	if err != nil {
		return err
	}

	return err
}

func TestImportCluster(t *testing.T) {
	suite.Run(t, new(ImportClusterTestSuite))
}
