package workloads

import (
	"fmt"
	"testing"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/namespaces"
	"github.com/rancher/shepherd/extensions/projects"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	testSession := session.NewSession()
	w.session = testSession

	client, err := rancher.NewClient("", testSession)
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

func (w *WorkloadTestSuite) TestWorkloadDaemonSet() {
	subSession := w.session.NewSession()
	defer subSession.Cleanup()

	clusterID, err := clusters.GetClusterIDByName(w.client, w.client.RancherConfig.ClusterName)
	require.NoError(w.T(), err)

	steveclient, err := w.client.Steve.ProxyDownstream(clusterID)
	require.NoError(w.T(), err)

	projectList, err := projects.ListProjectNames(w.client, clusterID)
	require.NoError(w.T(), err)

	project, err := projects.GetProjectByName(w.client, clusterID, projectList[0])
	require.NoError(w.T(), err)

	namespace, err := namespaces.CreateNamespace(w.client, namegen.AppendRandomString("namespacename"), "{}", map[string]string{}, map[string]string{}, project)
	require.NoError(w.T(), err)

	ngInxTemplate := workloads.NewContainer(namegen.AppendRandomString("nginx"), "nginx", corev1.PullAlways, nil, nil, nil, nil, nil)
	podTemplate := workloads.NewPodTemplate([]corev1.Container{ngInxTemplate}, nil, nil, nil)
	require.NoError(w.T(), err)

	deploymentTemplate := workloads.NewDeploymentTemplate(ngInxTemplate.Name, namespace.Name, podTemplate, true, nil)
	_, err = steveclient.SteveType(workloads.DeploymentSteveType).Create(deploymentTemplate)
	require.NoError(w.T(), err)

	daemonsetTemplate := workloads.NewDaemonSetTemplate(ngInxTemplate.Name, namespace.Name, podTemplate, true, nil)
	_, err = steveclient.SteveType(workloads.DaemonsetSteveType).Create(daemonsetTemplate)
	require.NoError(w.T(), err)

	time.Sleep(30 * time.Second)

	steveID := fmt.Sprintf(namespace.Name + "/" + ngInxTemplate.Name)
	daemonsetResp, err := steveclient.SteveType(workloads.DaemonsetSteveType).ByID(steveID)
	require.NoError(w.T(), err)

	daemonsetStatus := &appv1.DaemonSetStatus{}
	err = v1.ConvertToK8sType(daemonsetResp.Status, daemonsetStatus)
	require.NoError(w.T(), err)

	assert.Equalf(w.T(), 1, int(daemonsetStatus.NumberAvailable), "Daemonset doesn't have the required available")
	assert.Equalf(w.T(), 1, int(daemonsetStatus.NumberReady), "Daemonset doesn't have the required ready")
}

func TestWorkloadTestSuite(t *testing.T) {
	suite.Run(t, new(WorkloadTestSuite))
}
