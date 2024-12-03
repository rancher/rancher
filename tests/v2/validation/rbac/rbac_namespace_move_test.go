package rbac

import (
	"regexp"
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NameSpaceMoveTestSuite struct {
	suite.Suite
	client                      *rancher.Client
	standardUser                *management.User
	standardUserClient          *rancher.Client
	session                     *session.Session
	cluster                     *management.Cluster
	adminProject                *management.Project
	standardUserProjectOwner    *management.Project
	standardUserProjectMember   *management.Project
	standardUserProjectReadOnly *management.Project
	steveAdminClient            *v1.Client
	steveNonAdminClient         *v1.Client
	namespace                   string
	clusterName                 string
}

func (rb *NameSpaceMoveTestSuite) TearDownSuite() {
	rb.session.Cleanup()
}

func (rb *NameSpaceMoveTestSuite) SetupSuite() {
	testSession := session.NewSession()
	rb.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(rb.T(), err)

	rb.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rb.T(), clusterName, "Cluster name to install should be set")

	clusterID, err := clusters.GetClusterIDByName(rb.client, clusterName)
	require.NoError(rb.T(), err, "Error getting cluster ID")

	rb.cluster, err = rb.client.Management.Cluster.ByID(clusterID)
	require.NoError(rb.T(), err)

}

func (rb *NameSpaceMoveTestSuite) createProjectWithUserRole(role string) (*management.Project, error) {
	createdProject, err := createProject(rb.client, rb.cluster.ID)
	require.NoError(rb.T(), err)

	switch role {
	case roleProjectOwner:
		rb.standardUserProjectOwner = createdProject
		err = users.AddProjectMember(rb.client, rb.standardUserProjectOwner, rb.standardUser, role)

	case roleProjectMember:
		rb.standardUserProjectMember = createdProject
		err = users.AddProjectMember(rb.client, rb.standardUserProjectMember, rb.standardUser, role)

	case roleProjectReadOnly:
		rb.standardUserProjectReadOnly = createdProject
		err = users.AddProjectMember(rb.client, rb.standardUserProjectReadOnly, rb.standardUser, role)
	}

	return createdProject, err
}

func (rb *NameSpaceMoveTestSuite) createProjects() {
	newUser, err := users.CreateUserWithRole(rb.client, users.UserConfig(), standardUser)
	require.NoError(rb.T(), err)

	rb.standardUser = newUser
	rb.T().Logf("Created user: %v", rb.standardUser.Username)
	rb.standardUserClient, err = rb.client.AsUser(newUser)
	require.NoError(rb.T(), err)

	rb.createProjectWithUserRole(roleProjectOwner)
	rb.createProjectWithUserRole(roleProjectMember)
	rb.createProjectWithUserRole(roleProjectReadOnly)
}

func (rb *NameSpaceMoveTestSuite) createNameSpace(fromProject *management.Project, role string) (*v1.SteveAPIObject, error) {
	fromProjectName := namegen.AppendRandomString("testns-")
	fromProjectNameSpace, err := namespaces.CreateNamespace(rb.client, fromProjectName+"-project-namespace-"+role, "{}", map[string]string{}, map[string]string{}, fromProject)
	require.NoError(rb.T(), err)
	return fromProjectNameSpace, err
}

func moveNameSpaceFromAProjectRoleToAProject(rb *NameSpaceMoveTestSuite, fromProject *management.Project, fromRole string, toProject *management.Project, toRole string, standardUserClient *v1.Client, isSuccessful bool) {
	fromProjectNameSpace, err := rb.createNameSpace(fromProject, fromRole)
	toProjectNameSpace, err := rb.createNameSpace(toProject, toRole)

	steveAdminClient, err := rb.client.Steve.ProxyDownstream(rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.steveAdminClient = steveAdminClient

	updateNS, err := getAndConvertNamespace(fromProjectNameSpace, steveAdminClient)
	require.NoError(rb.T(), err)

	updateNS.Labels = toProjectNameSpace.Labels
	updateNS.Annotations = toProjectNameSpace.Annotations

	steveStdUserClient, err := rb.standardUserClient.Steve.ProxyDownstream(rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.steveNonAdminClient = steveStdUserClient

	if isSuccessful {
		convertedProjectNameSpace, err := rb.steveAdminClient.SteveType(namespaces.NamespaceSteveType).Update(fromProjectNameSpace, updateNS)
		require.NoError(rb.T(), err)
		assert.Equal(rb.T(), toProjectNameSpace.Annotations, convertedProjectNameSpace.Annotations)
	} else {
		_, err = rb.steveNonAdminClient.SteveType(namespaces.NamespaceSteveType).Update(toProjectNameSpace, updateNS)
		require.Error(rb.T(), err)

		errStatus := strings.Split(err.Error(), ".")[1]
		rgx := regexp.MustCompile(`\[(.*?)\]`)
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(rb.T(), "403 Forbidden", errorMsg[1])
	}
}

func (rb *NameSpaceMoveTestSuite) validateAdminCanMoveANamespaceFromPOToReadOnlyProject() {
	moveNameSpaceFromAProjectRoleToAProject(rb, rb.standardUserProjectOwner, roleProjectOwner, rb.standardUserProjectReadOnly, roleProjectReadOnly, rb.steveAdminClient, true)
}

func (rb *NameSpaceMoveTestSuite) validateAdminCanMoveANamespaceFromProjectMember() {
	moveNameSpaceFromAProjectRoleToAProject(rb, rb.standardUserProjectReadOnly, roleProjectReadOnly, rb.standardUserProjectOwner, roleProjectOwner, rb.steveAdminClient, true)
}

func (rb *NameSpaceMoveTestSuite) validateAdminCanMoveANamespaceFromPMToReadOnlyProject() {
	moveNameSpaceFromAProjectRoleToAProject(rb, rb.standardUserProjectMember, roleProjectMember, rb.standardUserProjectReadOnly, roleProjectReadOnly, rb.steveAdminClient, true)
}

func (rb *NameSpaceMoveTestSuite) validateAdminWithClusterOwnerCanMoveANamespaceFromProjectReadOnlyPermission() {
	moveNameSpaceFromAProjectRoleToAProject(rb, rb.standardUserProjectOwner, roleOwner, rb.standardUserProjectReadOnly, roleProjectReadOnly, rb.steveAdminClient, true)
}

func (rb *NameSpaceMoveTestSuite) validateProjectOwnerCannotMoveANamespaceToReadOnlyProject() {
	steveStdUserclient, err := rb.client.Steve.ProxyDownstream(rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.steveNonAdminClient = steveStdUserclient

	moveNameSpaceFromAProjectRoleToAProject(rb, rb.standardUserProjectOwner, roleProjectOwner, rb.standardUserProjectReadOnly, roleProjectReadOnly, rb.steveNonAdminClient, false)
}

func (rb *NameSpaceMoveTestSuite) validateProjectMemberCannotMoveANamespaceToReadOnlyProject() {
	steveStdUserclient, err := rb.client.Steve.ProxyDownstream(rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.steveNonAdminClient = steveStdUserclient

	moveNameSpaceFromAProjectRoleToAProject(rb, rb.standardUserProjectMember, roleProjectMember, rb.standardUserProjectReadOnly, roleProjectReadOnly, rb.steveNonAdminClient, false)
}

func (rb *NameSpaceMoveTestSuite) ValidateStandardUserProjectReadOnlyCannotMoveANamespaceWithReadOnlyProjectPermission() {
	steveStdUserclient, err := rb.client.Steve.ProxyDownstream(rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.steveNonAdminClient = steveStdUserclient

	moveNameSpaceFromAProjectRoleToAProject(rb, rb.standardUserProjectReadOnly, roleProjectReadOnly, rb.standardUserProjectReadOnly, roleProjectReadOnly, rb.steveNonAdminClient, false)
}

func (rb *NameSpaceMoveTestSuite) validateProjectReadOnlyCannotMoveANamespaceToReadOnlyProject() {
	steveStdUserclient, err := rb.client.Steve.ProxyDownstream(rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.steveNonAdminClient = steveStdUserclient

	moveNameSpaceFromAProjectRoleToAProject(rb, rb.standardUserProjectOwner, roleOwner, rb.standardUserProjectReadOnly, roleProjectReadOnly, rb.steveNonAdminClient, false)
}

func (rb *NameSpaceMoveTestSuite) TestMoveNamespaceToNewProjectTestSuite() {

	rb.Run("Create three projects with different roles", func() {
		rb.createProjects()
	})

	rb.Run("Validate if Admin user can move a namespace from a project owner project to aRead Only project", func() {
		rb.validateAdminCanMoveANamespaceFromPOToReadOnlyProject()
	})

	rb.Run("Validate if Admin user can move a namespace to a project member project", func() {
		rb.validateAdminCanMoveANamespaceFromProjectMember()
	})

	rb.Run("Validate if Admin user can move a namespace to a Read Only project", func() {
		rb.validateAdminCanMoveANamespaceFromPMToReadOnlyProject()
	})

	rb.Run("Validate if standard user with project owner permissions cannot move a namespace to a Read Only project", func() {
		rb.validateProjectOwnerCannotMoveANamespaceToReadOnlyProject()
	})

	rb.Run("Validate if standard user with project member permissions cannot move a namespace to a Read Only project", func() {
		rb.validateProjectMemberCannotMoveANamespaceToReadOnlyProject()
	})

	rb.Run("Validate if standard user with project read only permissions cannot move a namespace to a Read Only project", func() {
		rb.validateProjectReadOnlyCannotMoveANamespaceToReadOnlyProject()
	})

}

func TestNameSpaceMoveTestSuite(t *testing.T) {
	suite.Run(t, new(NameSpaceMoveTestSuite))
}
