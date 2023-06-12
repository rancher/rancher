package encryption

import (
	"fmt"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	nodestatus "github.com/rancher/rancher/tests/framework/extensions/nodes"
	"github.com/rancher/rancher/tests/framework/extensions/secrets"
	"github.com/rancher/rancher/tests/framework/extensions/sshkeys"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

type RKE2SecretEncryptionTestSuite struct {
	suite.Suite
	session *session.Session
	client  *rancher.Client
	ns      string
	sshUser string
}

func (s *RKE2SecretEncryptionTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

func (s *RKE2SecretEncryptionTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.ns = "default"
	sshCfg := &SSHConfig{}
	config.LoadConfig(SSHConfigFileKey, sshCfg)
	s.sshUser = sshCfg.User

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client
}

func (s *RKE2SecretEncryptionTestSuite) TestRKE2SecretEncryptionEnabled() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	require.NoError(s.T(), err)

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(s.T(), clusterName, "Cluster name is not provided")

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(s.T(), err)

	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), steveClient)

	podLabelKey := namegen.AppendRandomString("app")
	podLabelValue := namegen.AppendRandomString("encryption")
	podLables := map[string]string{podLabelKey: podLabelValue}

	secretObj, deploymentObj, err := createDeploymentWithSecret(steveClient, namegen.AppendRandomString("test-deployment"), namegen.AppendRandomString("test-secret"), s.ns, podLables)
	require.NoError(s.T(), err)

	require.NoError(s.T(), workloads.VerifyDeployment(steveClient, deploymentObj))
	logrus.Infof("deployment with secret created successfully!")

	query, err := url.ParseQuery(fmt.Sprintf("labelSelector=%s=%s", podLabelKey, podLables[podLabelKey]))
	require.NoError(s.T(), err)
	podObj, err := steveClient.SteveType(pods.PodResourceSteveType).List(query)
	require.NoError(s.T(), err)

	kubeConfig, err := kubeconfig.GetKubeconfig(client, clusterID)
	require.NoError(s.T(), err)

	restConfig, err := (*kubeConfig).ClientConfig()
	require.NoError(s.T(), err)

	cmdToReadEnvVar := []string{
		"/bin/sh",
		"-c",
		"echo $testKey",
	}

	err = kwait.Poll(5*time.Second, 2*time.Minute, func() (done bool, err error) {
		output, err := kubeconfig.KubectlExec(restConfig, podObj.Data[0].ObjectMeta.Name, s.ns, cmdToReadEnvVar)
		if err != nil {
			return false, nil
		}
		require.Equal(s.T(), output.String(), secretValue)
		logrus.Info("environment variable "+secretKey+" value : ", output.String())
		return true, nil
	})
	require.NoError(s.T(), err)

	nodesList, err := steveClient.SteveType("node").List(nil)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), nodesList)

	_, err = client.ReLogin()
	require.NoError(s.T(), err)

	for _, node := range nodesList.Data {
		etcdNodeLabel := node.Labels[nodestatus.EtcdNodeLabel]
		if etcdNodeLabel != "" {
			result, err := strconv.ParseBool(etcdNodeLabel)
			require.NoError(s.T(), err)
			if result && node.Annotations[nodestatus.ClusterAnnotation] == clusterName {
				machineName := node.Annotations[nodestatus.MachineAnnotation]
				sshKey, err := sshkeys.DownloadSSHKeys(client, machineName)
				require.NoError(s.T(), err)
				assert.NotEmpty(s.T(), sshKey)

				sshUser := s.sshUser
				assert.NotEmpty(s.T(), sshUser)

				clusterNode := &nodes.Node{
					NodeID:          node.ID,
					PublicIPAddress: node.Annotations[nodestatus.RKE2ExternalIP],
					SSHUser:         sshUser,
					SSHKey:          []byte(sshKey),
				}
				_, err = clusterNode.ExecuteCommand(EtcdCtlInstallCmd)
				require.NoError(s.T(), err)

				output, err := clusterNode.ExecuteCommand(fmt.Sprintf(rke2EtcdEncryptionCheckCmd, secretObj.Name))
				require.NoError(s.T(), err)
				require.NotEmpty(s.T(), output)
				// Verifying the secret encryption as per : https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/#verifying-that-data-is-encrypted
				require.Contains(s.T(), output, "k8s:enc:aescbc")
				logrus.Infof("secret encryption is enabled")
				break
			}
		}
	}
	assert.NoError(s.T(), steveClient.SteveType(secrets.SecretSteveType).Delete(secretObj))
	assert.NoError(s.T(), steveClient.SteveType(workloads.DeploymentSteveType).Delete(deploymentObj))
}

func TestRKE2SecretEncryption(t *testing.T) {
	suite.Run(t, new(RKE2SecretEncryptionTestSuite))
}
