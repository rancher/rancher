package integration

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	extnamespaces "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/namespaces"
	extrbac "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/secrets"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extauthz "github.com/rancher/shepherd/extensions/kubeapi/authorization"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/api/scheme"
	"github.com/rancher/shepherd/pkg/clientbase"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/suite"
	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	authzv1.SchemeBuilder.AddToScheme(scheme.Scheme.Scheme)
}

type RTBTestSuite struct {
	suite.Suite
	client              *rancher.Client
	project             *management.Project
	session             *session.Session
	downstreamClusterID string
}

func (p *RTBTestSuite) SetupSuite() {
	p.downstreamClusterID = "local"
	testSession := session.NewSession()
	p.session = testSession

	client, err := rancher.NewClient("", testSession)
	p.Require().NoError(err)

	p.client = client

	projectConfig := &management.Project{
		ClusterID: p.downstreamClusterID,
		Name:      "TestProject",
	}

	testProject, err := client.Management.Project.Create(projectConfig)
	p.Require().NoError(err)

	p.project = testProject
}

func (p *RTBTestSuite) TearDownSuite() {
	client, err := p.client.WithSession(p.session)
	p.Require().NoError(err)

	err = client.Management.Project.Delete(p.project)
	p.Require().NoError(err)
	p.session.Cleanup()
}

// newSubSession creates a new sub-session client for test isolation.
func (p *RTBTestSuite) newSubSession() *rancher.Client {
	subSession := p.session.NewSession()
	client, err := p.client.WithSession(subSession)
	p.Require().NoError(err)
	p.T().Cleanup(subSession.Cleanup)
	return client
}

// createUser creates a new user with the given global role and returns it with password set.
func (p *RTBTestSuite) createUser(client *rancher.Client, prefix, globalRole string) *management.User {
	enabled := true
	pw := password.GenerateUserPassword("testpass-")
	user, err := users.CreateUserWithRole(client, &management.User{
		Username: namegen.AppendRandomString(prefix + "-"),
		Password: pw,
		Name:     prefix,
		Enabled:  &enabled,
	}, globalRole)
	p.Require().NoError(err)
	user.Password = pw
	return user
}

// projectName extracts the project namespace name from a project ID (e.g. "local:p-xxxxx" → "p-xxxxx").
func (p *RTBTestSuite) projectName(project *management.Project) string {
	p.Require().NotNil(project)
	_, name, found := strings.Cut(project.ID, ":")
	p.Require().True(found, "projectName: invalid project ID %q, expected format <cluster>:<project>", project.ID)
	return name
}

// createNamespace creates a namespace in the given project with default settings.
func (p *RTBTestSuite) createNamespace(client *rancher.Client, projName string) *corev1.Namespace {
	ns, err := extnamespaces.CreateNamespace(client, p.downstreamClusterID, projName, namegen.AppendRandomString("testns-"), "{}", map[string]string{}, map[string]string{})
	p.Require().NoError(err)
	return ns
}

// assertClusterAccessRevoked verifies that the given user client no longer has access to the downstream cluster.
func (p *RTBTestSuite) assertClusterAccessRevoked(userClient *rancher.Client) {
	p.Require().Eventually(func() bool {
		clusters, err := userClient.Management.Cluster.List(nil)
		return err == nil && len(clusters.Data) == 0
	}, 2*time.Minute, 2*time.Second, "failed revoking cluster access from user")

	_, err := userClient.Management.Cluster.ByID(p.downstreamClusterID)
	p.Require().Error(err)
	p.Require().Contains(err.Error(), "403")
}

func (p *RTBTestSuite) TestPRTBRoleTemplateInheritance() {
	client := p.newSubSession()

	user := p.createUser(client, "testuser", "user")

	createdNamespace := p.createNamespace(client, p.projectName(p.project))

	testUser, err := client.AsUser(user)
	p.Require().NoError(err)

	// Test that user can get a specified secret once granted the permission to do so via roletemplate inheritance bounded
	// by a PRTB.

	secret, err := secrets.CreateSecretForCluster(client, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{GenerateName: "rtb-test-s-"}}, "local", createdNamespace.Name)
	p.Require().NoError(err)

	_, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, secret.Name, metav1.GetOptions{})
	p.Require().Error(err)

	rtB, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context: "project",
			Name:    "RoleB",
			Rules: []management.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"secrets"},
					ResourceNames: []string{secret.Name},
					Verbs:         []string{"get"},
				},
			},
		})
	p.Require().NoError(err)

	rtA, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context:         "project",
			Name:            "RoleA",
			RoleTemplateIDs: []string{rtB.ID},
		})
	p.Require().NoError(err)

	prtb, err := client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		ProjectID:       p.project.ID,
		UserPrincipalID: user.PrincipalIDs[0],
		RoleTemplateID:  rtA.ID,
	})
	p.Require().NoError(err)

	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:      "get",
			Resource:  "secrets",
			Name:      secret.Name,
			Namespace: createdNamespace.Name,
		},
	})
	p.Require().NoError(err)

	secret, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, secret.Name, metav1.GetOptions{})
	p.Require().NoError(err)

	err = client.Management.ProjectRoleTemplateBinding.Delete(prtb)
	p.Require().NoError(err)

	// Test that user can get a specified secret once granted the permission to do so via a chain of
	// roletemplate inheritance bounded by a PRTB. Here a chain means the permission is not directly inherited from the
	// parent.

	rtC, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context:         "project",
			Name:            "RoleC",
			RoleTemplateIDs: []string{rtA.ID},
		})
	p.Require().NoError(err)

	p.Require().Eventually(func() bool {
		_, err := secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, secret.Name, metav1.GetOptions{})
		return err != nil
	}, 2*time.Minute, 2*time.Second, "waiting for secret access to be revoked after PRTB removal")

	_, err = client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		ProjectID:       p.project.ID,
		UserPrincipalID: user.PrincipalIDs[0],
		RoleTemplateID:  rtC.ID,
	})
	p.Require().NoError(err)

	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:      "get",
			Resource:  "secrets",
			Name:      secret.Name,
			Namespace: createdNamespace.Name,
		},
	})
	p.Require().NoError(err)

	secret, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, secret.Name, metav1.GetOptions{})
	p.Require().NoError(err)

	anotherSecret, err := secrets.CreateSecretForCluster(client, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{GenerateName: "rtb-test-s-"}}, p.downstreamClusterID, createdNamespace.Name)
	p.Require().NoError(err)

	_, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, anotherSecret.Name, metav1.GetOptions{})
	p.Require().Error(err)

	// Test that permissions are updated when inherited roletemplate bound by PRTB is changed.

	updatedRTB := *rtB
	updatedRTB.Rules = append(rtB.Rules, management.PolicyRule{
		APIGroups:     []string{""},
		Resources:     []string{"secrets"},
		ResourceNames: []string{anotherSecret.Name},
		Verbs:         []string{"get"},
	})

	_, err = client.Management.RoleTemplate.Update(rtB, updatedRTB)
	p.Require().NoError(err)

	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:      "get",
			Resource:  "secrets",
			Name:      secret.Name,
			Namespace: createdNamespace.Name,
		},
		{
			Verb:      "get",
			Resource:  "secrets",
			Name:      anotherSecret.Name,
			Namespace: createdNamespace.Name,
		},
	})
	p.Require().NoError(err)

	_, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, anotherSecret.Name, metav1.GetOptions{})
	p.Require().NoError(err)
}

func (p *RTBTestSuite) TestCRTBRoleTemplateInheritance() {
	client := p.newSubSession()

	user := p.createUser(client, "testuser", "user")

	// Test that user can get a specified namespace once granted the permission to do so via roletemplate inheritance bounded
	// by a CRTB.

	pn := p.projectName(p.project)
	ns := p.createNamespace(client, pn)

	testUser, err := client.AsUser(user)
	p.Require().NoError(err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns.Name)
	p.Require().Error(err)

	rtB, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context: "",
			Name:    "RoleB",
			Rules: []management.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"namespaces"},
					ResourceNames: []string{ns.Name},
					Verbs:         []string{"get"},
				},
			},
		})
	p.Require().NoError(err)

	rtA, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context:         "cluster",
			Name:            "RoleA",
			RoleTemplateIDs: []string{rtB.ID},
		})
	p.Require().NoError(err)

	crtb, err := client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
		ClusterID:       p.downstreamClusterID,
		UserPrincipalID: user.PrincipalIDs[0],
		RoleTemplateID:  rtA.ID,
	})
	p.Require().NoError(err)

	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:     "get",
			Resource: "namespaces",
			Name:     ns.Name,
		},
	})
	p.Require().NoError(err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns.Name)
	p.Require().NoError(err)

	err = client.Management.ClusterRoleTemplateBinding.Delete(crtb)
	p.Require().NoError(err)

	// Ensure the user can no longer access the namespace after the CRTB is removed.
	p.Require().Eventually(func() bool {
		_, err := extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns.Name)
		return err != nil
	}, 2*time.Minute, 2*time.Second, "waiting for namespace access to be revoked after CRTB removal")

	// Test that user can get a specified namespace once granted the permission to do so via a chain of
	// roletemplate inheritance bounded by a CRTB. Here a chain means the permission is not directly inherited from the
	// parent.

	rtC, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context:         "cluster",
			Name:            "RoleC",
			RoleTemplateIDs: []string{rtA.ID},
		})
	p.Require().NoError(err)

	_, err = client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
		ClusterID:       p.downstreamClusterID,
		UserPrincipalID: user.PrincipalIDs[0],
		RoleTemplateID:  rtC.ID,
	})
	p.Require().NoError(err)

	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:     "get",
			Resource: "namespaces",
			Name:     ns.Name,
		},
	})
	p.Require().NoError(err)

	anotherNS := p.createNamespace(client, pn)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, anotherNS.Name)
	p.Require().Error(err)

	// Test that permissions are updated when inherited roletemplate bound by CRTB is changed.

	updatedRTB := *rtB
	updatedRTB.Rules = append(rtB.Rules, management.PolicyRule{
		APIGroups:     []string{""},
		Resources:     []string{"namespaces"},
		ResourceNames: []string{anotherNS.Name},
		Verbs:         []string{"get"},
	})

	_, err = client.Management.RoleTemplate.Update(rtB, updatedRTB)
	p.Require().NoError(err)

	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:     "get",
			Resource: "namespaces",
			Name:     ns.Name,
		},
		{
			Verb:     "get",
			Resource: "namespaces",
			Name:     anotherNS.Name,
		},
	})
	p.Require().NoError(err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, anotherNS.Name)
	p.Require().NoError(err)
}

func (p *RTBTestSuite) TestRemovingPRTBRevokesNamespaceAccess() {
	client := p.newSubSession()

	user := p.createUser(client, "testuser", "user")

	testUser, err := client.AsUser(user)
	p.Require().NoError(err)

	// Helper function to create a project and add the user as project-member
	createProjectAndAddUser := func() (*management.Project, *management.ProjectRoleTemplateBinding) {
		projectConfig := &management.Project{
			ClusterID: p.downstreamClusterID,
			Name:      namegen.AppendRandomString("test-project-"),
		}

		project, err := client.Management.Project.Create(projectConfig)
		p.Require().NoError(err)

		prtb, err := client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
			UserID:         user.ID,
			RoleTemplateID: "project-member",
			ProjectID:      project.ID,
		})
		p.Require().NoError(err)

		return project, prtb
	}

	// Create two projects and add user to both
	project1, _ := createProjectAndAddUser()
	project2, prtb2 := createProjectAndAddUser()

	// Helper function to add a namespace to a project
	addNamespaceToProject := func(project *management.Project) *corev1.Namespace {
		return p.createNamespace(client, p.projectName(project))
	}

	// Add namespace to first project
	ns1 := addNamespaceToProject(project1)

	// Verify user can access namespace in first project
	p.Require().Eventually(func() bool {
		_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns1.Name)
		return err == nil
	}, 2*time.Minute, 2*time.Second, "waiting for permissions to be applied to user")

	// Add namespace to second project
	ns2 := addNamespaceToProject(project2)

	// Verify user can access namespace in both projects
	p.Require().Eventually(func() bool {
		_, err1 := extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns1.Name)
		_, err2 := extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns2.Name)
		return err1 == nil && err2 == nil
	}, 2*time.Minute, 2*time.Second, "waiting for permissions to be applied to user")

	// Remove user from second project
	err = client.Management.ProjectRoleTemplateBinding.Delete(prtb2)
	p.Require().NoError(err)

	// Verify user can still access namespace in first project but not in second anymore
	p.Require().NoError(err)
	p.Require().Eventually(func() bool {
		_, err1 := extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns1.Name)
		_, err2 := extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns2.Name)
		return apierrors.IsForbidden(err2) && err1 == nil
	}, 2*time.Minute, 2*time.Second, "waiting for permissions to be removed from user")
}

func (p *RTBTestSuite) TestAPIGroupInRoleTemplate() {
	client := p.newSubSession()

	// Skip if admin can't see any nodes.
	adminNodes, err := client.Management.Node.List(nil)
	p.Require().NoError(err)
	if len(adminNodes.Data) == 0 {
		p.T().Skip("no nodes in the cluster")
	}

	user := p.createUser(client, "testuser", "user")

	testUser, err := client.AsUser(user)
	p.Require().NoError(err)

	// Validate the standard user cannot see any nodes initially.
	userNodes, err := testUser.Management.Node.List(nil)
	p.Require().NoError(err)
	p.Require().Empty(userNodes.Data, "standard user should not see any nodes")

	// Create a cluster-scoped role template with apiGroup-specific rules.
	rt, err := client.Management.RoleTemplate.Create(&management.RoleTemplate{
		Context: "cluster",
		Name:    namegen.AppendRandomString("test-rt-"),
		Rules: []management.PolicyRule{
			{
				APIGroups: []string{"management.cattle.io"},
				Resources: []string{"nodes", "nodepools"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"scheduling.k8s.io"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	})
	p.Require().NoError(err)

	// Wait for the role template to be available.
	p.Require().Eventually(func() bool {
		_, err := client.Management.RoleTemplate.ByID(rt.ID)
		return err == nil
	}, 2*time.Minute, 2*time.Second, "role template never became available")

	// Bind the user to the role template via a CRTB using the user's principal ID.
	p.Require().NotEmpty(user.PrincipalIDs, "test user has no principal IDs")
	_, err = client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
		UserPrincipalID: user.PrincipalIDs[0],
		RoleTemplateID:  rt.ID,
		ClusterID:       p.downstreamClusterID,
	})
	p.Require().NoError(err)

	// Wait for the user to be able to see nodes.
	p.Require().Eventually(func() bool {
		nodes, err := testUser.Management.Node.List(nil)
		return err == nil && len(nodes.Data) > 0
	}, 2*time.Minute, 2*time.Second, "user could never see nodes")

	// Verify user can see nodes.
	userNodes, err = testUser.Management.Node.List(nil)
	p.Require().NoError(err)
	p.Require().NotEmpty(userNodes.Data)

	// Verify user cannot delete a node (role only grants get/list/watch).
	err = testUser.Management.Node.Delete(&userNodes.Data[0])
	p.Require().ErrorContains(err, "403")
}

func (p *RTBTestSuite) TestDeletingPRTBRemovesClusterAccess() {
	client := p.newSubSession()

	user := p.createUser(client, "testuser", "user")

	testUser, err := client.AsUser(user)
	p.Require().NoError(err)

	// Admin creates a PRTB giving user project-member on the suite project.
	prtb, err := client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		UserID:         user.ID,
		RoleTemplateID: "project-member",
		ProjectID:      p.project.ID,
	})
	p.Require().NoError(err)

	// Verify the user can see the cluster.
	p.Require().Eventually(func() bool {
		_, err := testUser.Management.Cluster.ByID(p.downstreamClusterID)
		return err == nil
	}, 2*time.Minute, 2*time.Second, "user could never see the cluster")

	// Derive the label key from the PRTB ID (namespace:name -> namespace_name).
	// The label key is set on the membership CRB; the value varies by RBAC model
	// ("membership-binding-owner" in legacy, "true" with aggregation) so we use
	// a key-exists selector to cover both.
	prtbKey := strings.ReplaceAll(prtb.ID, ":", "_")

	// Wait for the membership ClusterRoleBinding to appear.
	p.Require().Eventually(func() bool {
		crbs, err := extrbac.ListClusterRoleBindings(client, p.downstreamClusterID, metav1.ListOptions{
			LabelSelector: prtbKey,
		})
		return err == nil && len(crbs.Items) == 1
	}, 2*time.Minute, 2*time.Second, fmt.Sprintf("failed waiting for clusterRoleBinding to get created with label %s for prtb %+v", prtbKey, prtb))

	// Delete the PRTB — user should lose access.
	err = client.Management.ProjectRoleTemplateBinding.Delete(prtb)
	p.Require().NoError(err)

	// Wait for the membership ClusterRoleBinding to be deleted.
	p.Require().Eventually(func() bool {
		crbs, err := extrbac.ListClusterRoleBindings(client, p.downstreamClusterID, metav1.ListOptions{
			LabelSelector: prtbKey,
		})
		return err == nil && len(crbs.Items) == 0
	}, 2*time.Minute, 2*time.Second, "failed waiting for clusterRoleBinding to get deleted")

	p.assertClusterAccessRevoked(testUser)
}

func (p *RTBTestSuite) TestDeletingPRTBCleansUpLegacyMembershipLabels() {
	client := p.newSubSession()

	user := p.createUser(client, "testuser", "user")

	testUser, err := client.AsUser(user)
	p.Require().NoError(err)

	// Admin creates a PRTB giving user project-member on the suite project.
	prtb, err := client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		UserID:         user.ID,
		RoleTemplateID: "project-member",
		ProjectID:      p.project.ID,
	})
	p.Require().NoError(err)

	// Verify the user can see the cluster.
	p.Require().Eventually(func() bool {
		_, err := testUser.Management.Cluster.ByID(p.downstreamClusterID)
		return err == nil
	}, 2*time.Minute, 2*time.Second, "user could never see the cluster")

	// The label key is set on the membership CRB; the value varies by RBAC model
	// so we use a key-exists selector to cover both legacy and aggregation.
	prtbKey := strings.ReplaceAll(prtb.ID, ":", "_")

	// Wait for the membership ClusterRoleBinding to appear.
	p.Require().Eventually(func() bool {
		crbs, err := extrbac.ListClusterRoleBindings(client, p.downstreamClusterID, metav1.ListOptions{
			LabelSelector: prtbKey,
		})
		return err == nil && len(crbs.Items) == 1
	}, 2*time.Minute, 2*time.Second, "failed waiting for clusterRoleBinding to get created")

	// Delete the PRTB — user should lose access and the membership CRB should be cleaned up.
	err = client.Management.ProjectRoleTemplateBinding.Delete(prtb)
	p.Require().NoError(err)

	// Wait for the membership ClusterRoleBinding to be gone.
	p.Require().Eventually(func() bool {
		crbs, err := extrbac.ListClusterRoleBindings(client, p.downstreamClusterID, metav1.ListOptions{
			LabelSelector: prtbKey,
		})
		return err == nil && len(crbs.Items) == 0
	}, 2*time.Minute, 2*time.Second, "failed waiting for cluster role bindings to be deleted")

	p.assertClusterAccessRevoked(testUser)
}

func (p *RTBTestSuite) TestCRTBCannotTargetUsersAndGroup() {
	client := p.newSubSession()

	user := p.createUser(client, "testuser", "user")

	_, err := client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
		Name:             namegen.AppendRandomString("crtb-"),
		ClusterID:        "local",
		UserID:           user.ID,
		GroupPrincipalID: "someauthprovidergroupid",
		RoleTemplateID:   "clustercatalogs-view",
	})
	p.Require().Error(err)

	var apiErr *clientbase.APIError
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)
	p.Require().Contains(apiErr.Body, "must target a user [userId]/[userPrincipalId] OR a group [groupId]/[groupPrincipalId]")
}

func (p *RTBTestSuite) TestCRTBMustHaveTarget() {
	client := p.newSubSession()

	_, err := client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
		Name:           namegen.AppendRandomString("crtb-"),
		ClusterID:      "local",
		RoleTemplateID: "clustercatalogs-view",
	})
	p.Require().Error(err)

	var apiErr *clientbase.APIError
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)
	p.Require().Contains(apiErr.Body, "must target a user [userId]/[userPrincipalId] OR a group [groupId]/[groupPrincipalId]")
}

func (p *RTBTestSuite) TestCRTBCannotUpdateSubjectsOrCluster() {
	client := p.newSubSession()

	user := p.createUser(client, "testuser", "user")

	oldCRTB, err := client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
		Name:           namegen.AppendRandomString("crtb-"),
		ClusterID:      "local",
		UserID:         user.ID,
		RoleTemplateID: "clustercatalogs-view",
	})
	p.Require().NoError(err)

	// Wait for userPrincipalId to be populated.
	p.Require().Eventually(func() bool {
		reloaded, err := client.Management.ClusterRoleTemplateBinding.ByID(oldCRTB.ID)
		if err != nil {
			return false
		}
		oldCRTB = reloaded
		return oldCRTB.UserPrincipalID != ""
	}, 2*time.Minute, 2*time.Second, "waiting for userPrincipalId to be populated")

	// Attempt to update immutable fields.
	updatedCRTB, err := client.Management.ClusterRoleTemplateBinding.Update(oldCRTB, map[string]interface{}{
		"clusterId":        "fakecluster",
		"userId":           "",
		"userPrincipalId":  "asdf",
		"groupPrincipalId": "asdf",
		"groupId":          "asdf",
	})
	p.Require().NoError(err)

	p.Require().Equal(oldCRTB.ClusterID, updatedCRTB.ClusterID)
	p.Require().Equal(oldCRTB.UserID, updatedCRTB.UserID)
	p.Require().Equal(oldCRTB.UserPrincipalID, updatedCRTB.UserPrincipalID)
	p.Require().Equal(oldCRTB.GroupPrincipalID, updatedCRTB.GroupPrincipalID)
	p.Require().Equal(oldCRTB.GroupID, updatedCRTB.GroupID)
}

func TestRTBTestSuite(t *testing.T) {
	suite.Run(t, new(RTBTestSuite))
}
