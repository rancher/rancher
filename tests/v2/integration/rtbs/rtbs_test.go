package integration

import (
	"context"
	"encoding/json"
	"fmt"
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
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func init() {
	authzv1.SchemeBuilder.AddToScheme(scheme.Scheme.Scheme)
}

type RTBTestSuite struct {
	suite.Suite
	testUser            *management.User
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
	require.NoError(p.T(), err)

	p.client = client

	projectConfig := &management.Project{
		ClusterID: p.downstreamClusterID,
		Name:      "TestProject",
	}

	testProject, err := client.Management.Project.Create(projectConfig)
	require.NoError(p.T(), err)

	p.project = testProject

	p.testUser = p.createUser(client, "testuser", "user")
}

func (p *RTBTestSuite) TearDownSuite() {
	client, err := p.client.WithSession(p.session)
	require.NoError(p.T(), err)

	// Clean up the project and user we created
	err = client.Management.Project.Delete(p.project)
	require.NoError(p.T(), err)
	err = client.Management.User.Delete(p.testUser)
	require.NoError(p.T(), err)
	p.session.Cleanup()
}

// newSubSession creates a new sub-session client for test isolation.
func (p *RTBTestSuite) newSubSession() (*rancher.Client, func()) {
	subSession := p.session.NewSession()
	client, err := p.client.WithSession(subSession)
	require.NoError(p.T(), err)
	return client, subSession.Cleanup
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
	require.NoError(p.T(), err)
	user.Password = pw
	return user
}

// projectName extracts the project namespace name from a project ID (e.g. "local:p-xxxxx" → "p-xxxxx").
func (p *RTBTestSuite) projectName(project *management.Project) string {
	require.NotNil(p.T(), project)
	_, name, found := strings.Cut(project.ID, ":")
	require.True(p.T(), found, "projectName: invalid project ID %q, expected format <cluster>:<project>", project.ID)
	return name
}

// createNamespace creates a namespace in the given project with default settings.
func (p *RTBTestSuite) createNamespace(client *rancher.Client, projName string) *corev1.Namespace {
	ns, err := extnamespaces.CreateNamespace(client, p.downstreamClusterID, projName, namegen.AppendRandomString("testns-"), "{}", map[string]string{}, map[string]string{})
	require.NoError(p.T(), err)
	return ns
}

// assertClusterAccessRevoked verifies that the given user client no longer has access to the downstream cluster.
func (p *RTBTestSuite) assertClusterAccessRevoked(userClient *rancher.Client) {
	require.Eventually(p.T(), func() bool {
		clusters, err := userClient.Management.Cluster.List(nil)
		return err == nil && len(clusters.Data) == 0
	}, 2*time.Minute, 2*time.Second, "failed revoking cluster access from user")

	_, err := userClient.Management.Cluster.ByID(p.downstreamClusterID)
	require.Error(p.T(), err)
	require.Contains(p.T(), err.Error(), "403")
}

func (p *RTBTestSuite) TestPRTBRoleTemplateInheritance() {
	client, cleanup := p.newSubSession()
	defer cleanup()

	createdNamespace := p.createNamespace(client, p.projectName(p.project))

	testUser, err := client.AsUser(p.testUser)
	require.NoError(p.T(), err)

	// Test that user can get a specified secret once granted the permission to do so via roletemplate inheritance bounded
	// by a PRTB.

	secret, err := secrets.CreateSecretForCluster(client, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{GenerateName: "rtb-test-s-"}}, "local", createdNamespace.Name)
	require.NoError(p.T(), err)

	_, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, secret.Name, metav1.GetOptions{})
	require.Error(p.T(), err)

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
	require.NoError(p.T(), err)

	rtA, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context:         "project",
			Name:            "RoleA",
			RoleTemplateIDs: []string{rtB.ID},
		})
	require.NoError(p.T(), err)

	err = users.AddProjectMember(client, p.project, p.testUser, rtA.ID, []*authzv1.ResourceAttributes{
		{
			Verb:      "get",
			Resource:  "secrets",
			Name:      secret.Name,
			Namespace: createdNamespace.Name,
		},
	})
	require.NoError(p.T(), err)

	secret, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, secret.Name, metav1.GetOptions{})
	require.NoError(p.T(), err)

	err = users.RemoveProjectMember(client, p.testUser)
	require.NoError(p.T(), err)

	// Test that user can get a specified secret once granted the permission to do so via a chain of
	// roletemplate inheritance bounded by a PRTB. Here a chain means the permission is not directly inherited from the
	// parent.

	rtC, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context:         "project",
			Name:            "RoleC",
			RoleTemplateIDs: []string{rtA.ID},
		})
	require.NoError(p.T(), err)

	_, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, secret.Name, metav1.GetOptions{})
	require.Error(p.T(), err)

	err = users.AddProjectMember(client, p.project, p.testUser, rtC.ID, []*authzv1.ResourceAttributes{
		{
			Verb:      "get",
			Resource:  "secrets",
			Name:      secret.Name,
			Namespace: createdNamespace.Name,
		},
	})
	require.NoError(p.T(), err)

	secret, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, secret.Name, metav1.GetOptions{})
	require.NoError(p.T(), err)

	anotherSecret, err := secrets.CreateSecretForCluster(client, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{GenerateName: "rtb-test-s-"}}, p.downstreamClusterID, createdNamespace.Name)
	require.NoError(p.T(), err)

	_, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, anotherSecret.Name, metav1.GetOptions{})
	require.Error(p.T(), err)

	// Test that permissions are updated when inherited roletemplate bound by PRTB is changed.

	updatedRTB := *rtB
	updatedRTB.Rules = append(rtB.Rules, management.PolicyRule{
		APIGroups:     []string{""},
		Resources:     []string{"secrets"},
		ResourceNames: []string{anotherSecret.Name},
		Verbs:         []string{"get"},
	})

	_, err = client.Management.RoleTemplate.Update(rtB, updatedRTB)
	require.NoError(p.T(), err)

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
	require.NoError(p.T(), err)

	_, err = secrets.GetSecretByName(testUser, p.downstreamClusterID, createdNamespace.Name, anotherSecret.Name, metav1.GetOptions{})
	require.NoError(p.T(), err)
}

func (p *RTBTestSuite) TestCRTBRoleTemplateInheritance() {
	client, cleanup := p.newSubSession()
	defer cleanup()

	// Test that user can get a specified namespace once granted the permission to do so via roletemplate inheritance bounded
	// by a CRTB.

	pn := p.projectName(p.project)
	ns := p.createNamespace(client, pn)

	testUser, err := client.AsUser(p.testUser)
	require.NoError(p.T(), err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns.Name)
	require.Error(p.T(), err)

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
	require.NoError(p.T(), err)

	rtA, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context:         "cluster",
			Name:            "RoleA",
			RoleTemplateIDs: []string{rtB.ID},
		})
	require.NoError(p.T(), err)

	localCluster, err := p.client.Management.Cluster.ByID(p.downstreamClusterID)
	require.NoError(p.T(), err)

	err = users.AddClusterRoleToUser(client, localCluster, p.testUser, rtA.ID, []*authzv1.ResourceAttributes{
		{
			Verb:     "get",
			Resource: "namespaces",
			Name:     ns.Name,
		},
	})
	require.NoError(p.T(), err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns.Name)
	require.NoError(p.T(), err)

	err = users.RemoveClusterRoleFromUser(client, p.testUser)
	require.NoError(p.T(), err)

	// Test that user can get a specified namespace once granted the permission to do so via a chain of
	// roletemplate inheritance bounded by a CRTB. Here a chain means the permission is not directly inherited from the
	// parent.

	rtC, err := client.Management.RoleTemplate.Create(
		&management.RoleTemplate{
			Context:         "cluster",
			Name:            "RoleC",
			RoleTemplateIDs: []string{rtA.ID},
		})
	require.NoError(p.T(), err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns.Name)
	require.Error(p.T(), err)

	err = users.AddClusterRoleToUser(client, localCluster, p.testUser, rtC.ID, []*authzv1.ResourceAttributes{
		{
			Verb:     "get",
			Resource: "namespaces",
			Name:     ns.Name,
		},
	})
	require.NoError(p.T(), err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns.Name)
	require.NoError(p.T(), err)

	anotherNS := p.createNamespace(client, pn)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, anotherNS.Name)
	require.Error(p.T(), err)

	// Test that permissions are updated when inherited roletemplate bound by CRTB is changed.

	updatedRTB := *rtB
	updatedRTB.Rules = append(rtB.Rules, management.PolicyRule{
		APIGroups:     []string{""},
		Resources:     []string{"namespaces"},
		ResourceNames: []string{anotherNS.Name},
		Verbs:         []string{"get"},
	})

	_, err = client.Management.RoleTemplate.Update(rtB, updatedRTB)
	require.NoError(p.T(), err)

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
	require.NoError(p.T(), err)

	_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, anotherNS.Name)
	require.NoError(p.T(), err)
}

func (p *RTBTestSuite) TestRemovingPRTBRevokesNamespaceAccess() {
	client, cleanup := p.newSubSession()
	defer cleanup()

	testUser, err := client.AsUser(p.testUser)
	require.NoError(p.T(), err)

	// Helper function to create a project and add the user as project-member
	createProjectAndAddUser := func() (*management.Project, *management.ProjectRoleTemplateBinding) {
		projectConfig := &management.Project{
			ClusterID: p.downstreamClusterID,
			Name:      namegen.AppendRandomString("test-project-"),
		}

		project, err := client.Management.Project.Create(projectConfig)
		require.NoError(p.T(), err)

		prtb, err := client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
			UserID:         p.testUser.ID,
			RoleTemplateID: "project-member",
			ProjectID:      project.ID,
		})
		require.NoError(p.T(), err)

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
	require.Eventually(p.T(), func() bool {
		_, err = extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns1.Name)
		return err == nil
	}, 2*time.Minute, 2*time.Second, "waiting for permissions to be applied to user")

	// Add namespace to second project
	ns2 := addNamespaceToProject(project2)

	// Verify user can access namespace in both projects
	require.Eventually(p.T(), func() bool {
		_, err1 := extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns1.Name)
		_, err2 := extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns2.Name)
		return err1 == nil && err2 == nil
	}, 2*time.Minute, 2*time.Second, "waiting for permissions to be applied to user")

	// Remove user from second project
	err = client.Management.ProjectRoleTemplateBinding.Delete(prtb2)
	require.NoError(p.T(), err)

	// Verify user can still access namespace in first project but not in second anymore
	require.NoError(p.T(), err)
	require.Eventually(p.T(), func() bool {
		_, err1 := extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns1.Name)
		_, err2 := extnamespaces.GetNamespaceByName(testUser, p.downstreamClusterID, ns2.Name)
		return apierrors.IsForbidden(err2) && err1 == nil
	}, 2*time.Minute, 2*time.Second, "waiting for permissions to be removed from user")
}

func (p *RTBTestSuite) TestAPIGroupInRoleTemplate() {
	client, cleanup := p.newSubSession()
	defer cleanup()

	// Skip if admin can't see any nodes.
	adminNodes, err := client.Management.Node.List(nil)
	require.NoError(p.T(), err)
	if len(adminNodes.Data) == 0 {
		p.T().Skip("no nodes in the cluster")
	}

	testUser, err := client.AsUser(p.testUser)
	require.NoError(p.T(), err)

	// Validate the standard user cannot see any nodes initially.
	userNodes, err := testUser.Management.Node.List(nil)
	require.NoError(p.T(), err)
	require.Empty(p.T(), userNodes.Data, "standard user should not see any nodes")

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
	require.NoError(p.T(), err)

	// Wait for the role template to be available.
	require.Eventually(p.T(), func() bool {
		_, err := client.Management.RoleTemplate.ByID(rt.ID)
		return err == nil
	}, 2*time.Minute, 2*time.Second, "role template never became available")

	// Bind the user to the role template via a CRTB using the user's principal ID.
	require.NotEmpty(p.T(), p.testUser.PrincipalIDs, "test user has no principal IDs")
	_, err = client.Management.ClusterRoleTemplateBinding.Create(&management.ClusterRoleTemplateBinding{
		UserPrincipalID: p.testUser.PrincipalIDs[0],
		RoleTemplateID:  rt.ID,
		ClusterID:       p.downstreamClusterID,
	})
	require.NoError(p.T(), err)

	// Wait for the user to be able to see nodes.
	require.Eventually(p.T(), func() bool {
		nodes, err := testUser.Management.Node.List(nil)
		return err == nil && len(nodes.Data) > 0
	}, 2*time.Minute, 2*time.Second, "user could never see nodes")

	// Verify user can see nodes.
	userNodes, err = testUser.Management.Node.List(nil)
	require.NoError(p.T(), err)
	require.NotEmpty(p.T(), userNodes.Data)

	// Verify user cannot delete a node (role only grants get/list/watch).
	err = testUser.Management.Node.Delete(&userNodes.Data[0])
	require.Error(p.T(), err)
	require.Contains(p.T(), err.Error(), "403")
}

func (p *RTBTestSuite) TestDeletingPRTBRemovesClusterAccess() {
	client, cleanup := p.newSubSession()
	defer cleanup()

	testUser, err := client.AsUser(p.testUser)
	require.NoError(p.T(), err)

	mbo := "membership-binding-owner"

	// Admin creates a PRTB giving user project-member on the suite project.
	prtb, err := client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		UserID:         p.testUser.ID,
		RoleTemplateID: "project-member",
		ProjectID:      p.project.ID,
	})
	require.NoError(p.T(), err)

	// Verify the user can see the cluster.
	require.Eventually(p.T(), func() bool {
		_, err := testUser.Management.Cluster.ByID(p.downstreamClusterID)
		return err == nil
	}, 2*time.Minute, 2*time.Second, "user could never see the cluster")

	// Derive the label key from the PRTB ID (namespace:name -> namespace_name).
	prtbKey := strings.ReplaceAll(prtb.ID, ":", "_")

	// Wait for the expected ClusterRoleBinding with the membership-binding-owner label.
	require.Eventually(p.T(), func() bool {
		crbs, err := extrbac.ListClusterRoleBindings(client, p.downstreamClusterID, metav1.ListOptions{
			LabelSelector: prtbKey + "=" + mbo,
		})
		return err == nil && len(crbs.Items) == 1
	}, 2*time.Minute, 2*time.Second, fmt.Sprintf("failed waiting for clusterRoleBinding to get created with label %s for prtb %+v", prtbKey, prtb))

	// Delete the PRTB — user should lose access.
	err = client.Management.ProjectRoleTemplateBinding.Delete(prtb)
	require.NoError(p.T(), err)

	// Wait for the ClusterRoleBinding to be deleted.
	require.Eventually(p.T(), func() bool {
		crbs, err := extrbac.ListClusterRoleBindings(client, p.downstreamClusterID, metav1.ListOptions{
			LabelSelector: prtbKey + "=" + mbo,
		})
		return err == nil && len(crbs.Items) == 0
	}, 2*time.Minute, 2*time.Second, "failed waiting for clusterRoleBinding to get deleted")

	p.assertClusterAccessRevoked(testUser)
}

func (p *RTBTestSuite) TestDeletingPRTBCleansUpLegacyMembershipLabels() {
	client, cleanup := p.newSubSession()
	defer cleanup()

	testUser, err := client.AsUser(p.testUser)
	require.NoError(p.T(), err)

	mbo := "membership-binding-owner"
	// Intentionally misspelled — this is how the label was spelled prior to 2.5.
	mboLegacy := "memberhsip-binding-owner"

	// Admin creates a PRTB giving user project-member on the suite project.
	prtb, err := client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		UserID:         p.testUser.ID,
		RoleTemplateID: "project-member",
		ProjectID:      p.project.ID,
	})
	require.NoError(p.T(), err)

	// Verify the user can see the cluster.
	require.Eventually(p.T(), func() bool {
		_, err := testUser.Management.Cluster.ByID(p.downstreamClusterID)
		return err == nil
	}, 2*time.Minute, 2*time.Second, "user could never see the cluster")

	prtbKey := strings.ReplaceAll(prtb.ID, ":", "_")

	// Wait for the CRB with the new-style label.
	require.Eventually(p.T(), func() bool {
		crbs, err := extrbac.ListClusterRoleBindings(client, p.downstreamClusterID, metav1.ListOptions{
			LabelSelector: prtbKey + "=" + mbo,
		})
		return err == nil && len(crbs.Items) == 1
	}, 2*time.Minute, 2*time.Second, "failed waiting for clusterRoleBinding to get created")

	// Fetch the CRB to patch it with the legacy label.
	crbs, err := extrbac.ListClusterRoleBindings(client, p.downstreamClusterID, metav1.ListOptions{
		LabelSelector: prtbKey + "=" + mbo,
	})
	require.NoError(p.T(), err)
	require.Len(p.T(), crbs.Items, 1)

	// Patch the CRB to add the legacy label (using PRTB UUID as key) to simulate a pre-2.5 upgrade.
	patchPayload, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"labels": map[string]string{
				prtb.UUID: mboLegacy,
			},
		},
	})
	require.NoError(p.T(), err)

	dynamicClient, err := client.GetDownStreamClusterClient(p.downstreamClusterID)
	require.NoError(p.T(), err)

	crbResource := dynamicClient.Resource(extrbac.ClusterRoleBindingGroupVersionResource)
	_, err = crbResource.Patch(context.TODO(), crbs.Items[0].Name, k8stypes.StrategicMergePatchType, patchPayload, metav1.PatchOptions{})
	require.NoError(p.T(), err)

	// Wait for the legacy label to appear.
	require.Eventually(p.T(), func() bool {
		crbs, err := extrbac.ListClusterRoleBindings(client, p.downstreamClusterID, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", prtb.UUID, mboLegacy),
		})
		return err == nil && len(crbs.Items) == 1
	}, 2*time.Minute, 2*time.Second, "failed waiting for legacy label to be applied")

	// Delete the PRTB — user should lose access and both labels should be cleaned up.
	err = client.Management.ProjectRoleTemplateBinding.Delete(prtb)
	require.NoError(p.T(), err)

	// Wait for CRBs with both the new and legacy labels to be gone.
	require.Eventually(p.T(), func() bool {
		newCRBs, err1 := extrbac.ListClusterRoleBindings(client, p.downstreamClusterID, metav1.ListOptions{
			LabelSelector: prtbKey + "=" + mbo,
		})
		legacyCRBs, err2 := extrbac.ListClusterRoleBindings(client, p.downstreamClusterID, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", prtb.UUID, mboLegacy),
		})
		return err1 == nil && err2 == nil && len(newCRBs.Items) == 0 && len(legacyCRBs.Items) == 0
	}, 2*time.Minute, 2*time.Second, "failed waiting for cluster role bindings to be deleted")

	p.assertClusterAccessRevoked(testUser)
}

func TestRTBTestSuite(t *testing.T) {
	suite.Run(t, new(RTBTestSuite))
}
