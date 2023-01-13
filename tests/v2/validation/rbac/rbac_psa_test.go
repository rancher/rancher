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
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	coreV1 "k8s.io/api/core/v1"
)

type PSATestSuite struct {
	suite.Suite
	client              *rancher.Client
	nonAdminUser        *management.User
	nonAdminUserClient  *rancher.Client
	session             *session.Session
	cluster             *management.Cluster
	adminProject        *management.Project
	steveAdminClient    *v1.Client
	steveNonAdminClient *v1.Client
	adminNamespace      *v1.SteveAPIObject
}

func (rb *PSATestSuite) TearDownSuite() {
	rb.session.Cleanup()
}

func (rb *PSATestSuite) SetupSuite() {
	testSession := session.NewSession(rb.T())
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

func (rb *PSATestSuite) ConvertNS(namespace *v1.SteveAPIObject) *coreV1.Namespace {
	getNSSteveObject, err := rb.steveAdminClient.SteveType(namespaces.NamespaceSteveType).ByID(namespace.ID)
	require.NoError(rb.T(), err)

	namespaceObj := &coreV1.Namespace{}
	err = v1.ConvertToK8sType(getNSSteveObject.JSONResp, namespaceObj)
	require.NoError(rb.T(), err)

	return namespaceObj
}

func (rb *PSATestSuite) ValidatePSA(role string) {

	labels := map[string]string{
		psaWarn:    pssPrivilegedPolicy,
		psaEnforce: pssPrivilegedPolicy,
		psaAudit:   pssPrivilegedPolicy,
	}

	rb.T().Logf("Validate updating the PSA labels as %v", role)

	//Testcase1 Validate updating the PSA labels
	updateNS := rb.ConvertNS(rb.adminNamespace)
	updateNS.Labels = labels

	response, err := rb.steveNonAdminClient.SteveType(namespaces.NamespaceSteveType).Update(rb.adminNamespace, updateNS)

	switch role {
	case restrictedAdmin, roleOwner:
		require.NoError(rb.T(), err)
		expectedLabels := getPSALabels(response, labels)
		assert.Equal(rb.T(), labels, expectedLabels)
	case roleMember, roleProjectOwner, roleProjectMember, roleProjectReadOnly:
		require.Error(rb.T(), err)
		if role == roleMember || role == roleProjectReadOnly {
			errMessage := strings.Split(err.Error(), ":")[0]
			assert.Equal(rb.T(), "Resource type [namespace] is not updatable", errMessage)
		} else {
			errStatus := strings.Split(err.Error(), ".")[1]
			rgx := regexp.MustCompile(`\[(.*?)\]`)
			errorMsg := rgx.FindStringSubmatch(errStatus)
			assert.Equal(rb.T(), "403 Forbidden", errorMsg[1])
		}
	}

	rb.T().Logf("Validate deletion of the PSA labels as %v", role)

	for label := range labels {
		if strings.Contains(label, psaWarn) || strings.Contains(label, psaAudit) || strings.Contains(label, psaEnforce) {
			delete(labels, label)
		}
	}

	deleteLabelsNS := rb.ConvertNS(rb.adminNamespace)
	deleteLabelsNS.Labels = labels

	_, err = rb.steveNonAdminClient.SteveType(namespaces.NamespaceSteveType).Update(rb.adminNamespace, deleteLabelsNS)
	switch role {
	case restrictedAdmin, roleOwner:
		require.NoError(rb.T(), err)
		expectedLabels := getPSALabels(response, labels)
		assert.Equal(rb.T(), 0, len(expectedLabels))
		_, err = createDeployment(rb.steveNonAdminClient, rb.client, rb.cluster.ID, "nginx", "nginx", rb.adminNamespace.Name)
		require.NoError(rb.T(), err)
	case roleMember, roleProjectOwner, roleProjectMember, roleProjectReadOnly:
		require.Error(rb.T(), err)
		if role == roleMember || role == roleProjectReadOnly {
			errMessage := strings.Split(err.Error(), ":")[0]
			assert.Equal(rb.T(), "Resource type [namespace] is not updatable", errMessage)
		} else {
			errStatus := strings.Split(err.Error(), ".")[1]
			rgx := regexp.MustCompile(`\[(.*?)\]`)
			errorMsg := rgx.FindStringSubmatch(errStatus)
			assert.Equal(rb.T(), "403 Forbidden", errorMsg[1])
		}
	}

	rb.T().Logf("Validate creation of new namespace with PSA labels as %v", role)

	labels = map[string]string{
		psaWarn:    pssBaselinePolicy,
		psaEnforce: pssBaselinePolicy,
		psaAudit:   pssBaselinePolicy,
	}
	namespaceName := namegenerator.AppendRandomString("testns-")
	namespaceCreate, err := namespaces.CreateNamespace(rb.nonAdminUserClient, namespaceName, "{}", labels, map[string]string{}, rb.adminProject)

	switch role {
	case restrictedAdmin, roleOwner:
		require.NoError(rb.T(), err)
		expectedLabels := getPSALabels(response, labels)
		assert.Equal(rb.T(), labels, expectedLabels)
		_, err = createDeployment(rb.steveNonAdminClient, rb.client, rb.cluster.ID, "nginx", "nginx", namespaceCreate.Name)
		require.NoError(rb.T(), err)
	case roleMember, roleProjectOwner, roleProjectMember, roleProjectReadOnly:
		require.Error(rb.T(), err)
		if role == roleMember || role == roleProjectReadOnly {
			errMessage := strings.Split(err.Error(), ":")[0]
			assert.Equal(rb.T(), "Resource type [namespace] is not creatable", errMessage)
		} else {
			errStatus := strings.Split(err.Error(), ".")[1]
			rgx := regexp.MustCompile(`\[(.*?)\]`)
			errorMsg := rgx.FindStringSubmatch(errStatus)
			assert.Equal(rb.T(), "403 Forbidden", errorMsg[1])
		}
	}

}

func (rb *PSATestSuite) ValidateAdditionalPSA(role string) {

	createProjectAsNonAdmin, err := createProject(rb.nonAdminUserClient, rb.cluster.ID)
	require.NoError(rb.T(), err)

	namespaceName := namegen.AppendRandomString("testns-")
	createNamespace, err := namespaces.CreateNamespace(rb.nonAdminUserClient, namespaceName, "{}",
		map[string]string{}, map[string]string{},
		createProjectAsNonAdmin)
	require.NoError(rb.T(), err)

	rb.T().Logf("Validate editing new namespace in a cluster member created project with PSA labels as %v", role)
	labels := map[string]string{
		psaWarn:    pssRestrictedPolicy,
		psaEnforce: pssRestrictedPolicy,
		psaAudit:   pssRestrictedPolicy,
	}

	updateNS := rb.ConvertNS(createNamespace)
	updateNS.Labels = labels

	relogin, err := rb.nonAdminUserClient.ReLogin()
	require.NoError(rb.T(), err)
	rb.nonAdminUserClient = relogin

	steveStdUserclient, err := rb.nonAdminUserClient.Steve.ProxyDownstream(rb.cluster.ID)
	require.NoError(rb.T(), err)
	rb.steveNonAdminClient = steveStdUserclient

	response, err := rb.steveNonAdminClient.SteveType(namespaces.NamespaceSteveType).Update(createNamespace, updateNS)

	switch role {
	case roleOwner:
		require.NoError(rb.T(), err)
		expectedLabels := getPSALabels(response, labels)
		assert.Equal(rb.T(), labels, expectedLabels)
		_, err = createDeployment(rb.steveNonAdminClient, rb.client, rb.cluster.ID, "nginx", "nginx", createNamespace.Name)
		require.Error(rb.T(), err)
	case roleMember:
		require.Error(rb.T(), err)
		errStatus := strings.Split(err.Error(), ".")[1]
		rgx := regexp.MustCompile(`\[(.*?)\]`)
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(rb.T(), "403 Forbidden", errorMsg[1])

		//Edit the namespace and add PSA labels as an admin for a cluster member
		updateNS := rb.ConvertNS(createNamespace)
		updateNS.Labels = labels
		_, err := rb.steveAdminClient.SteveType(namespaces.NamespaceSteveType).Update(createNamespace, updateNS)
		require.NoError(rb.T(), err)

	}

	rb.T().Logf("Validate deletion of PSA labels in namespace in a cluster member created project as %v", role)

	for label := range labels {
		if strings.Contains(label, psaWarn) || strings.Contains(label, psaAudit) || strings.Contains(label, psaEnforce) {
			delete(labels, label)
		}
	}

	deleteLabelsNS := rb.ConvertNS(createNamespace)
	deleteLabelsNS.Labels = labels

	_, err = rb.steveNonAdminClient.SteveType(namespaces.NamespaceSteveType).Update(createNamespace, deleteLabelsNS)

	switch role {
	case roleOwner:
		require.NoError(rb.T(), err)
		expectedLabels := getPSALabels(response, labels)
		assert.Equal(rb.T(), labels, expectedLabels)
		_, err = createDeployment(rb.steveNonAdminClient, rb.client, rb.cluster.ID, "nginx", "nginx", createNamespace.Name)
		require.NoError(rb.T(), err)
	case roleMember:
		require.Error(rb.T(), err)
		errStatus := strings.Split(err.Error(), ".")[1]
		rgx := regexp.MustCompile(`\[(.*?)\]`)
		errorMsg := rgx.FindStringSubmatch(errStatus)
		assert.Equal(rb.T(), "403 Forbidden", errorMsg[1])
	}

}

func (rb *PSATestSuite) TestPSA() {
	nonAdminUserRoles := [...]string{roleMember, roleOwner, restrictedAdmin, roleProjectOwner, roleProjectMember, roleProjectReadOnly} 
	for _, role := range nonAdminUserRoles {

		//As an admin, create a namespace by adding the PSA labels.
		rb.Run("Add PSA labels on the namespaces created by admins ", func() {
			createProjectAsAdmin, err := createProject(rb.client, rb.cluster.ID)
			rb.adminProject = createProjectAsAdmin
			require.NoError(rb.T(), err)

			steveAdminClient, err := rb.client.Steve.ProxyDownstream(rb.cluster.ID)
			require.NoError(rb.T(), err)
			rb.steveAdminClient = steveAdminClient

			namespaceName := namegenerator.AppendRandomString("testns-")

			labels := map[string]string{
				psaWarn:    pssRestrictedPolicy,
				psaEnforce: pssRestrictedPolicy,
				psaAudit:   pssRestrictedPolicy,
			}
			adminNamespace, err := namespaces.CreateNamespace(rb.client, namespaceName+"-admin", "{}", labels, map[string]string{}, rb.adminProject)
			require.NoError(rb.T(), err)
			expectedPSALabels := getPSALabels(adminNamespace, labels)
			assert.Equal(rb.T(), labels, expectedPSALabels)
			rb.adminNamespace = adminNamespace
			//Create a deployment and verify the deployment fails
			_, err = createDeployment(rb.steveAdminClient, rb.client, rb.cluster.ID, "nginx", "nginx", rb.adminNamespace.Name)
			require.Error(rb.T(), err)
		})

		rb.Run("Create a user with global role "+role, func() {
			var userRole string
			if role == restrictedAdmin {
				userRole = restrictedAdmin
			} else {
				userRole = "user"
			}
			newUser, err := createUser(rb.client, userRole)
			require.NoError(rb.T(), err)
			rb.nonAdminUser = newUser
			rb.T().Logf("Created user: %v", rb.nonAdminUser.Username)
			rb.nonAdminUserClient, err = rb.client.AsUser(newUser)
			require.NoError(rb.T(), err)

			subSession := rb.session.NewSession()
			defer subSession.Cleanup()

			//Adding created user to the downstream clusters with the role cluster Owner.
			log.Info("Adding user as " + role + " to the downstream cluster.")
			if role != restrictedAdmin {
				if strings.Contains(role, "project") || role == roleProjectReadOnly {
					err := users.AddProjectMember(rb.client, rb.adminProject, rb.nonAdminUser, role)
					require.NoError(rb.T(), err)
				} else {
					err := users.AddClusterRoleToUser(rb.client, rb.cluster, rb.nonAdminUser, role)
					require.NoError(rb.T(), err)
				}

				rb.nonAdminUserClient, err = rb.nonAdminUserClient.ReLogin()
				require.NoError(rb.T(), err)

			}

			steveClient, err := rb.nonAdminUserClient.Steve.ProxyDownstream(rb.cluster.ID)
			require.NoError(rb.T(), err)
			rb.steveNonAdminClient = steveClient
		})

		rb.Run("Testcase - Validate if members with roles "+role+"can add/edit/delete labesl from admin created namespace", func() {
			rb.ValidatePSA(role)
		})

		if strings.Contains(role, "cluster") {
			rb.Run("Additional testcase - Validate if members with roles "+role+"can add/edit/delete labels from admin created namespace", func() {
				rb.ValidateAdditionalPSA(role)
			})
		}
	}

}

func TestPSATestSuite(t *testing.T) {
	suite.Run(t, new(PSATestSuite))
}
