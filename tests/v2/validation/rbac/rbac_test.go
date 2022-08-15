package rbac

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	//"time"

	"github.com/rancher/norman/types"
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
	clusterList, err := client.Provisioning.Cluster.List(&types.ListOpts{}) 

	// assert the count of clusters obtained as the user is same as the expected cluster count
	//assert cluster list contains right cluster ID
	if err == nil{
		actualResult := len(clusterList.Data)
		assert.Equal(rb.T(), expectedClusterCount, actualResult)
			if clusterID != "" {
				assert.Equal(rb.T(), clusterID, clusterList.Data[0].Status.ClusterName )
			}
		}
	return err
}


func (rb *rbTestSuite) ValidateClusterOwner() {

	//Testcase1 Standard users should return empty list of clusters before they are added as cluster owner.
	rb.T().Log("Testcase1 - Validating standard users cannot list any downstream clusters")
	err := rb.ValidateClustersList(rb.standardUserClient,0, "")
	require.Error(rb.T(),err)
	assert.Equal(rb.T(), "Resource type [provisioning.cattle.io.cluster] is not listable", err.Error())
	err = users.AddClusterMember(rb.client, rb.cluster, rb.standardUser, "cluster-owner")
	require.NoError(rb.T(), err)
	rb.standardUserClient, err = rb.standardUserClient.ReLogin()
	require.NoError(rb.T(), err)
	rb.T().Log("Validation successful!")
	

	//Testcase2 Cluster owner should list all the clusters they are owner of
	rb.T().Log("Testcase2 - Validating the cluster count obtained as a cluster owner")
	err = rb.ValidateClustersList(rb.standardUserClient, 1, rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.T().Log("Validation successful!")


	//Testcase3 Cluster Owner should be able to list all the projects in a cluster
	rb.T().Log("Testcase3 - Validating cluster owner is able to list all projects")
	//Create a project as an admin

	createProjectAsAdmin, err := CreateProject(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.T().Log("Created project",createProjectAsAdmin.Name)
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
	rb.T().Log("Validation successful!")

	
	//Testcase4 Cluster Owner should be able to create a project in a cluster
	rb.T().Log("Testcase4 - Validating cluster owner is able to create a project in the cluster")
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), createProjectAsAdmin.Name, createProjectAsAdmin.Name)
	actualStatus := fmt.Sprintf("%v", createProjectAsAdmin.State)
	assert.Equal(rb.T(),"active",actualStatus)
	rb.T().Log("Validation successful!")

		
	//Testcase5 Cluster Owner should be able to create namespace in project they are not owner of
	rb.T().Log("Testcase5 - Validating cluster owner can create namespace in a project they are not owner of")
	namespaceName := AppendRandomString("testns-")	
	createdNamespace, err := namespaces.CreateNamespace(rb.standardUserClient, namespaceName, "{}", map[string]string{}, map[string]string{}, createProjectAsAdmin)
	require.NoError(rb.T(), err)
	assert.Equal(rb.T(), namespaceName, createdNamespace.Name)
	actualStatus = fmt.Sprintf("%v", createdNamespace.Status.Phase)
	assert.Equal(rb.T(), "Active", actualStatus)
	rb.T().Log("Validation successful!")
	

	//Testcase6 Cluster Owner should be able to list all the namespaces in a cluster
	rb.T().Log("Testcase6 - Validating cluster owner lists all namespaces in a cluster")
	//Get the list of namespaces as and admin client
	namespaceListAdmin, err := GetListNamespaces(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	//Get the list of namespaces as and admin client
	namespaceListClusterOwner, err := GetListNamespaces(rb.standardUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	//Length of namespace list for admin and cluster owner should match
	assert.Equal(rb.T(), len(namespaceListAdmin), len(namespaceListClusterOwner))
	//Namespaces obtained as admin and cluster owner should be same
	assert.Equal(rb.T(), namespaceListAdmin, namespaceListClusterOwner)
	rb.T().Log("Validation successful!")


	//Testcase7 Cluster Owner should be able to delete the namespace they are not owner of
	err = namespaces.DeleteNamespace(rb.standardUserClient,namespaceName, rb.cluster.ID )
	require.NoError(rb.T(),err)
	rb.T().Log("Validation successful!")

	//Testcase8 Cluster Owner should be able to delete the project they are not owner of
	err = projects.DeleteProject(rb.standardUserClient, createProjectAsAdmin)
	require.NoError(rb.T(), err)
	rb.T().Log("Validation successful!")
}


func (rb *rbTestSuite) ValidateClusterMember() {

	err := users.AddClusterMember(rb.client, rb.cluster, rb.standardUser, "cluster-member")
	require.NoError(rb.T(), err)
	rb.standardUserClient, err = rb.standardUserClient.ReLogin()
	require.NoError(rb.T(), err)

	//Testcase1 Cluster Member should list all the clusters they are member of
	rb.T().Log("Validating the cluster count obtained as a cluster member")
	err = rb.ValidateClustersList(rb.standardUserClient, 1, rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.T().Log("Validation successful!")


	//Testcase2 Cluster Member should not be able to list the projects in a cluster
	// they are not owner of
	rb.T().Log("Validating cluster member is not able to list projects")
	 createProjectAsAdmin, err := CreateProject(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)
	//Get project list as a cluster member
	projectlistClusterMember, err := GetListProjects(rb.standardUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)
	//assert projects list obtained as a cluster member is empty
	assert.Equal(rb.T(), 0, len(projectlistClusterMember))
	rb.T().Log("Validation successful!")

	
	
	//Testcase3 Cluster member should not be able to create namespace in project they are not owner of
	rb.T().Log("Validating cluster member cannot create namespace in a project they are not owner of")
	namespaceName := AppendRandomString("testns-")
	_, err = namespaces.CreateNamespace(rb.standardUserClient, namespaceName, "{}", map[string]string{}, map[string]string{}, createProjectAsAdmin)
	require.Error(rb.T(), err)
	//assert cluster member gets an error when creating a namespace in a project they are not owner of
	errMessage := strings.Split(err.Error(), ":")[0]
	assert.Equal(rb.T(),"namespaces is forbidden",errMessage)
	rb.T().Log("Validation successful!")


	//Testcase4 Cluster member should not be able to list all the namespaces in a cluster
	rb.T().Log("Validating cluster member cannot lists all namespaces in a cluster")
	//Get the list of namespaces as and admin client
	namespacelistClusterMember, err := GetListNamespaces(rb.standardUserClient, rb.cluster.ID)
	require.Error(rb.T(), err)
	errMessage = strings.Split(err.Error(), ":")[0]
	//assert namespaces list obtained as a cluster member is empty
	assert.Equal(rb.T(), len(namespacelistClusterMember), 0)
	//assert cluster member is not able to list namespaces
	assert.Equal(rb.T(), "namespaces is forbidden",errMessage)
	rb.T().Log("Validation successful!")


	//Testcase5 Cluster Member should not be able to delete the project they are not owner of
	err = projects.DeleteProject(rb.standardUserClient, createProjectAsAdmin)
	require.Error(rb.T(), err)
	errStatus := strings.Split(err.Error(), ".")[1]
	rgx := regexp.MustCompile(`\[(.*?)\]`)
	errorMsg := rgx.FindStringSubmatch(errStatus)
	assert.Equal(rb.T(), "403 Forbidden" , errorMsg[1])
	rb.T().Log("Validation successful!")
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

		rb.ValidateClusterOwner()

		//Remove added cluster owner from the cluster as an admin
		rb.T().Log("Validating cluster owner is removed from the cluster and returns nil clusters")
		err = users.RemoveClusterMember(rb.client, rb.standardUser)
		require.NoError(rb.T(), err)


	})
}


func (rb *rbTestSuite) TestRbClusterMember() {

	rb.Run("ClusterMember", func() {
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

		rb.ValidateClusterMember()

		//Remove added cluster member from the cluster as an admin
		rb.T().Log("Validating cluster member is removed from the cluster and returns nil clusters")
		err = users.RemoveClusterMember(rb.client, rb.standardUser)
		require.NoError(rb.T(), err)


	})
}


func TestRBAC(t *testing.T) {
	suite.Run(t, new(rbTestSuite))
}
