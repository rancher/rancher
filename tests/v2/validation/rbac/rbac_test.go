package rbac

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/projects"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


type rbTestSuite struct {
	suite.Suite
	client             *rancher.Client
	standardUser       *management.User
	standardUserClient *rancher.Client
	session            *session.Session
	cluster            *management.Cluster
}


func (rb *rbTestSuite) TearDownSuite() {
	rb.session.Cleanup()
}

func (rb *rbTestSuite) SetupSuite() {
	testSession := session.NewSession(rb.T())
	rb.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(rb.T(), err)

	rb.client = client

	//Get cluster name from the config file and append cluster details in rb
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rb.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rb.client, clusterName)
	require.NoError(rb.T(), err, "Error getting cluster ID")
	rb.cluster, err = rb.client.Management.Cluster.ByID(clusterID)
	require.NoError(rb.T(), err)

}

func (rb *rbTestSuite) ValidateClustersList(client *rancher.Client, expectedClusterCount int, clusterID string) (err error) {
	clusterList, err := client.Provisioning.Clusters("").List(context.TODO(), metav1.ListOptions{})
	require.NoError(rb.T(),err)

	// assert the count of clusters obtained as the user is same as the expected cluster count
	//assert cluster list contains right cluster ID
	actualResult := len(clusterList.Items)
	assert.Equal(rb.T(), expectedClusterCount, actualResult)
	if clusterID != "" {
		assert.Equal(rb.T(), clusterID, clusterList.Items[0].Status.ClusterName, )
	}
	return err
}


func (rb *rbTestSuite) ValidateClusterOwner() {

	//Testcase Cluster owner should list all the clusters they are owner of
	rb.T().Log("Validating the cluster count obtained as a cluster owner")
	err := rb.ValidateClustersList(rb.standardUserClient, 1, rb.cluster.ID)
	require.NoError(rb.T(), err)


	//Testcase Cluster Owner should be able to list all the projects in a cluster
	rb.T().Log("Validating cluster owner is able to list all projects")
	//Create a project as an admin
	projectConfig := &management.Project{
		ClusterID: rb.cluster.ID,
		Name:      AppendRandomString("testproject-"),
	}
	testProject, err := rb.client.Management.Project.Create(projectConfig)
	require.NoError(rb.T(), err)

	
	//Get project list as an admin
	projectlistAdmin, err := GetListProjects(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)

	//Get project list as a cluster owner
	projectlistClusterOwner, err := GetListProjects(rb.standardUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)

	//assert length of projects list obtained as an admin to the cluster owner is equal
	assert.Equal(rb.T(), len(projectlistAdmin), len(projectlistClusterOwner))

	//assert projects list obtained as an admin to the cluster owner is the same
	assert.Equal(rb.T(), projectlistAdmin, projectlistClusterOwner)

	
	
	//Testcase Cluster Owner should be able to create namespace in project they are not owner of
	rb.T().Log("Validating cluster owner can create namespace in a project they are not owner of")

	namespaceName := AppendRandomString("testns-")
	if err == nil {
		createdNamespace, err := namespaces.CreateNamespace(rb.standardUserClient, namespaceName, "{}", map[string]string{}, map[string]string{}, testProject)
		require.NoError(rb.T(), err)
		assert.Equal(rb.T(), namespaceName, createdNamespace.Name, )
		actual_status := fmt.Sprintf("%v", createdNamespace.Status.Phase)
		assert.Equal(rb.T(), "Active", actual_status, )

	}

	//Testcase Cluster Owner should be able to list all the namespaces in a cluster
	rb.T().Log("Validating cluster owner lists all namespaces in a cluster")

	//Get the list of namespaces as and admin client
	namespacelist_admin, err := GetListNamespaces(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)

	//Get the list of namespaces as and admin client
	namespacelist_cluster_owner, err := GetListNamespaces(rb.standardUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)

	//Length of namespace list for admin and cluster owner should match
	assert.Equal(rb.T(), len(namespacelist_admin), len(namespacelist_cluster_owner))

	//Namespaces obtained as admin and cluster owner should be same
	assert.Equal(rb.T(), namespacelist_admin, namespacelist_cluster_owner)


	//Testcase Cluster Owner should be able to delete the namespace they are not owner of
	err = namespaces.DeleteNamespace(rb.standardUserClient,namespaceName, rb.cluster.ID )
	require.NoError(rb.T(),err)

	//Testcase Cluster Owner should be able to delete the project they are not owner of
	err = projects.DeleteProject(rb.standardUserClient, testProject)
	require.NoError(rb.T(), err)
}



func (rb *rbTestSuite) TestRbClusterOwner() {

	rb.Run("ClusterOwner", func() {
		newUser, err := CreateUser(rb.client)
		require.NoError(rb.T(), err)
		rb.standardUser = newUser
		rb.T().Log("Created user", rb.standardUser.Username)

		rb.standardUserClient, err = rb.client.AsUser(newUser)
		require.NoError(rb.T(), err)

		
		subSession := rb.session.NewSession()
		defer subSession.Cleanup()
		_, err = rb.standardUserClient.WithSession(subSession)

		require.NoError(rb.T(), err)

		//Standard users should return empty list of clusters before they are added as cluster owner.
		err = rb.ValidateClustersList(rb.standardUserClient ,0, "")

		if err == nil {

			err = users.AddClusterMember(rb.client, rb.cluster, rb.standardUser, "cluster-owner")
			require.NoError(rb.T(), err)
		}
		time.Sleep(2 * time.Second)
		rb.ValidateClusterOwner()

		//Remove added cluster owner from the cluster as an admin
		rb.T().Log("Validating cluster owner is removed from the cluster and returns nil clusters")
		err = users.RemoveClusterMember(rb.client, rb.standardUser)
		require.NoError(rb.T(), err)

		time.Sleep(2 * time.Second)
		//Validate cluster owner cannot list any more clusters
		err = rb.ValidateClustersList(rb.standardUserClient ,0, "")
		require.NoError(rb.T(), err)

	})
}

func TestRbClusterOwner(t *testing.T) {
	suite.Run(t, new(rbTestSuite))
}
