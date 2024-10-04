//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package workloads

import (
	"testing"

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

func (w *WorkloadTestSuite) TearDownSuite() {
	w.session.Cleanup()
}

func (w *WorkloadTestSuite) SetupSuite() {
	w.session = session.NewSession()

	client, err := rancher.NewClient("", w.session)
	require.NoError(w.T(), err)

	w.client = client

	log.Info("Getting cluster name from the config file and append cluster details in connection")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(w.T(), clusterName, "Cluster name to install should be set")

	clusterID, err := clusters.GetClusterIDByName(w.client, clusterName)
	require.NoError(w.T(), err, "Error getting cluster ID")

	w.cluster, err = w.client.Management.Cluster.ByID(clusterID)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadDeployment() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	err := WorkloadDeploymentTest(w.client, w.cluster.ID)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadSideKick() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	err := WorkloadSideKickTest(w.client, w.cluster.ID)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadDaemonSet() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	err := WorkloadDaemonSetTest(w.client, w.cluster.ID)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadCronjob() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	err := WorkloadCronjobTest(w.client, w.cluster.ID)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadStatefulset() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	err := WorkloadStatefulsetTest(w.client, w.cluster.ID)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadUpgrade() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	err := WorkloadUpgradeTest(w.client, w.cluster.ID)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadPodScaleUp() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	err := WorkloadPodScaleUpTest(w.client, w.cluster.ID)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadPodScaleDown() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	err := WorkloadPodScaleDownTest(w.client, w.cluster.ID)
	require.NoError(w.T(), err)
}

func (w *WorkloadTestSuite) TestWorkloadPauseOrchestration() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	err := WorkloadPauseOrchestrationTest(w.client, w.cluster.ID)
	require.NoError(w.T(), err)
}

// NOTE: this is an example DO NOT MERGE
func (w *WorkloadTestSuite) TestDemoSubTestCase() {
	//Do some test i.e provision a cluster for OS-checks, snapshot test, normal provisioning test... etc
	// ---
	// ---
	// ---
	//As part of the above test we now want to chain this with the network checks,
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	workloadTests := GetAllWorkloadTests()
	for _, workloadTest := range workloadTests {
		w.Suite.Run(workloadTest.name, func() {
			err := workloadTest.testFunc(w.client, w.cluster.ID)
			require.NoError(w.T(), err)
		})
	}
}

func TestWorkloadTestSuite(t *testing.T) {
	suite.Run(t, new(WorkloadTestSuite))
}
