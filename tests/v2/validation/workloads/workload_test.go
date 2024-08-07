package workloads

import (
	"testing"

	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type WorkloadTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (workload *WorkloadTestSuite) TearDownSuite() {
	workload.session.Cleanup()
}

func (workload *WorkloadTestSuite) SetupSuite() {
	workload.session = session.NewSession()

	client, err := rancher.NewClient("", workload.session)
	require.NoError(workload.T(), err)

	workload.client = client

	log.Info("Getting cluster name from the config file and append cluster details in connection")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(workload.T(), clusterName, "Cluster name to install should be set")

	clusterID, err := clusters.GetClusterIDByName(workload.client, clusterName)
	require.NoError(workload.T(), err, "Error getting cluster ID")

	workload.cluster, err = workload.client.Management.Cluster.ByID(clusterID)
	require.NoError(workload.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadDeployment() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	_, namespace, err := projectsapi.CreateProjectAndNamespace(w.client, w.cluster.ID)
	require.NoError(w.T(), err)

	_, err = deployment.CreateDeployment(w.client, w.cluster.ID, namespace.Name, 1, "", "", false, false)
	require.NoError(w.T(), err)
}

func TestWorkloadTestSuite(t *testing.T) {
	suite.Run(t, new(WorkloadTestSuite))
}
