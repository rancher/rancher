//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package projects

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/constants"
	secretsApi "github.com/rancher/rancher/tests/framework/extensions/kubeapi/secrets"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProjectScopedSecretTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (pss *ProjectScopedSecretTestSuite) TearDownSuite() {
	pss.session.Cleanup()
}

func (pss *ProjectScopedSecretTestSuite) SetupSuite() {
	pss.session = session.NewSession()

	client, err := rancher.NewClient("", pss.session)
	require.NoError(pss.T(), err)

	pss.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(pss.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(pss.client, clusterName)
	require.NoError(pss.T(), err, "Error getting cluster ID")
	pss.cluster, err = pss.client.Management.Cluster.ByID(clusterID)
	require.NoError(pss.T(), err)

}

func (pss *ProjectScopedSecretTestSuite) testProjectScopedSecret(clusterID string) (*v3.Project, *corev1.Secret, []*corev1.Namespace) {
	log.Info("Create a project in the cluster.")
	createdProject, err := createProject(pss.client, clusterID)
	require.NoError(pss.T(), err)
	pss.T().Logf("Created project: %v", createdProject.Name)

	log.Info("Create a project scoped secret in the project.")
	createdProjectScopedSecret, err := createProjectScopedSecret(pss.client, createdProject)
	require.NoError(pss.T(), err)
	pss.T().Logf("Created project scoped secret: %v", createdProjectScopedSecret.Name)

	log.Info("Verify that the project scoped secret has the expected label \"cattle.io/project-scoped: original\" and annotation \"field.cattle.io/projectId: cluster-id:project-id\".")
	annotationValue := createdProject.Namespace + ":" + createdProject.Name
	err = validateProjectSecretLabelsAndAnnotations(createdProjectScopedSecret, annotationValue)
	require.NoError(pss.T(), err)

	log.Info("Create two namespaces in the project.")
	namespaceCount := 2
	namespaceList, err := createNamespaces(pss.client, namespaceCount, createdProject)
	require.NoError(pss.T(), err)

	log.Info("Verify that the secret is propagated to both the namespaces in the project and create a deployment using the propagated secret.")
	err = validatePropagatedNamespaceSecret(pss.client, createdProjectScopedSecret, namespaceList)
	require.NoError(pss.T(), err)

	return createdProject, createdProjectScopedSecret, namespaceList
}

func (pss *ProjectScopedSecretTestSuite) TestProjectScopedSecretLocalCluster() {
	subSession := pss.session.NewSession()
	defer subSession.Cleanup()

	pss.testProjectScopedSecret(constants.LocalCluster)
}

func (pss *ProjectScopedSecretTestSuite) TestProjectScopedSecretDownstreamCluster() {
	subSession := pss.session.NewSession()
	defer subSession.Cleanup()

	pss.testProjectScopedSecret(pss.cluster.ID)
}

func (pss *ProjectScopedSecretTestSuite) TestProjectScopedSecretAfterCreatingNamespace() {
	subSession := pss.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a project in the cluster.")
	createdProject, err := createProject(pss.client, pss.cluster.ID)
	require.NoError(pss.T(), err)
	pss.T().Logf("Created project: %v", createdProject.Name)

	log.Info("Create two namespaces in the project.")
	namespaceCount := 2
	namespaceList, err := createNamespaces(pss.client, namespaceCount, createdProject)
	require.NoError(pss.T(), err)

	log.Info("Create a project scoped secret in the project.")
	createdProjectScopedSecret, err := createProjectScopedSecret(pss.client, createdProject)
	require.NoError(pss.T(), err)
	pss.T().Logf("Created project scoped secret: %v", createdProjectScopedSecret.Name)

	log.Info("Verify that the project scoped secret has the expected label \"cattle.io/project-scoped: original\" and annotation \"field.cattle.io/projectId: cluster-id:project-id\".")
	annotationValue := createdProject.Namespace + ":" + createdProject.Name
	err = validateProjectSecretLabelsAndAnnotations(createdProjectScopedSecret, annotationValue)
	require.NoError(pss.T(), err)

	log.Info("Verify that the secret is propagated to both the namespaces in the project and create a deployment using the propagated secret.")
	err = validatePropagatedNamespaceSecret(pss.client, createdProjectScopedSecret, namespaceList)
	require.NoError(pss.T(), err)

}

func (pss *ProjectScopedSecretTestSuite) TestUpdateProjectScopedSecret() {
	subSession := pss.session.NewSession()
	defer subSession.Cleanup()

	_, createdProjectScopedSecret, namespaceList := pss.testProjectScopedSecret(pss.cluster.ID)

	log.Info("Update the secret data in the Project scoped secret.")
	newData := map[string][]byte{
		"foo": []byte("bar"),
	}

	createdProjectScopedSecret.Data = newData
	updatedProjectScopedSecret, err := secretsApi.UpdateSecret(pss.client, constants.LocalCluster, createdProjectScopedSecret)
	require.NoError(pss.T(), err)

	log.Info("Verify that the secret data in the project scoped secret has been updated.")
	require.Equal(pss.T(), newData, updatedProjectScopedSecret.Data, "Secret data is not as expected")
	annotationValue := updatedProjectScopedSecret.Namespace + ":" + updatedProjectScopedSecret.Name
	err = validateProjectSecretLabelsAndAnnotations(updatedProjectScopedSecret, annotationValue)
	require.NoError(pss.T(), err)

	log.Info("Verify that the updated secret is propagated to all the namespaces in the project and create a deployment using the updated secret.")
	err = validatePropagatedNamespaceSecret(pss.client, updatedProjectScopedSecret, namespaceList)
	require.NoError(pss.T(), err)
}

func (pss *ProjectScopedSecretTestSuite) TestUpdateSecretInNamespaceAfterSecretPropagation() {
	subSession := pss.session.NewSession()
	defer subSession.Cleanup()

	_, createdProjectScopedSecret, namespaceList := pss.testProjectScopedSecret(pss.cluster.ID)

	log.Info("Update the secret data in one of the namespaces's secret.")
	newData := map[string][]byte{
		"foo": []byte("bar"),
	}
	namespaceSecret, err := secretsApi.GetSecretByName(pss.client, pss.cluster.ID, namespaceList[0].Name, createdProjectScopedSecret.Name, metav1.GetOptions{})
	require.NoError(pss.T(), err)

	namespaceSecret.Data = newData
	_, err = secretsApi.UpdateSecret(pss.client, pss.cluster.ID, namespaceSecret)
	require.NoError(pss.T(), err)

	log.Info("Verify that the secret data is reverted back to the secret data that was propagated from the project.")
	err = validatePropagatedNamespaceSecret(pss.client, createdProjectScopedSecret, namespaceList)
	require.NoError(pss.T(), err)
}

func TestProjectScopedSecretTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectScopedSecretTestSuite))
}
