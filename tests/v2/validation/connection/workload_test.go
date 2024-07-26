package connection

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/extensions/workloads/deployment"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
)

type ConnectionTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (connection *ConnectionTestSuite) TearDownSuite() {
	connection.session.Cleanup()
}

func (connection *ConnectionTestSuite) SetupSuite() {
	testSession := session.NewSession()
	connection.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(connection.T(), err)

	connection.client = client

	log.Info("Getting cluster name from the config file and append cluster details in connection")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(connection.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(connection.client, clusterName)
	require.NoError(connection.T(), err, "Error getting cluster ID")
	connection.cluster, err = connection.client.Management.Cluster.ByID(clusterID)
	require.NoError(connection.T(), err)
}

func (c *ConnectionTestSuite) TestWorkloadSideKick() {
	subSession := c.session.NewSession()
	defer subSession.Cleanup()

	clusterID, err := clusters.GetClusterIDByName(c.client, c.client.RancherConfig.ClusterName)
	require.NoError(c.T(), err)

	steveclient, err := c.client.Steve.ProxyDownstream(clusterID)
	require.NoError(c.T(), err)

	ngInxTemplate := workloads.NewContainer(namegen.AppendRandomString("nginx"), "nginx", corev1.PullAlways, []corev1.VolumeMount{}, []corev1.EnvFromSource{}, nil, nil, nil)
	ngInxDeployment, err := createDeployment(steveclient, []corev1.Container{ngInxTemplate}, ngInxTemplate.Name)
	require.NoError(c.T(), err)

	err = deployment.VerifyDeployment(steveclient, ngInxDeployment)
	require.NoError(c.T(), err)

	redisTemplate := workloads.NewContainer(namegen.AppendRandomString("redis"), "redis", corev1.PullAlways, []corev1.VolumeMount{}, []corev1.EnvFromSource{}, nil, nil, nil)
	redisDeployment, err := createDeployment(steveclient, []corev1.Container{redisTemplate}, redisTemplate.Name)
	require.NoError(c.T(), err)

	err = deployment.VerifyDeployment(steveclient, redisDeployment)
	require.NoError(c.T(), err)

	err = deployment.VerifyDeployment(steveclient, ngInxDeployment)
	require.NoError(c.T(), err)
}

func TestConnectionTestSuite(t *testing.T) {
	suite.Run(t, new(ConnectionTestSuite))
}
