//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package workloads

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/workloads/cronjob"
	"github.com/rancher/rancher/tests/v2/actions/workloads/daemonset"
	"github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/rancher/tests/v2/actions/workloads/statefulset"
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

func (w *WorkloadTestSuite) TestWorkloads() {
	workloadTests := []struct {
		name     string
		testFunc func(client *rancher.Client, clusterID string) error
	}{
		{"WorkloadDeploymentTest", deployment.VerifyCreateDeployment},
		{"WorkloadSideKickTest", deployment.VerifyCreateDeploymentSideKick},
		{"WorkloadDaemonSetTest", daemonset.VerifyCreateDaemonSet},
		{"WorkloadCronjobTest", cronjob.VerifyCreateCronjob},
		{"WorkloadStatefulsetTest", statefulset.VerifyCreateStatefulset},
		{"WorkloadUpgradeTest", deployment.VerifyDeploymentUpgradeRollback},
		{"WorkloadPodScaleUpTest", deployment.VerifyDeploymentPodScaleUp},
		{"WorkloadPodScaleDownTest", deployment.VerifyDeploymentPodScaleDown},
		{"WorkloadPauseOrchestrationTest", deployment.VerifyDeploymentPauseOrchestration},
	}

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
