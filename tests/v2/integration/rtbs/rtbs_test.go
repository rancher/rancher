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
	extunstructured "github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/api/scheme"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

func (p *RTBTestSuite) TearDownSuite() {
	p.session.Cleanup()
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

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(p.T(), err)
	newUser.Password = user.Password
	p.testUser = newUser
}

func (p *RTBTestSuite) TestPRTBRoleTemplateInheritance() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	client, err := p.client.WithSession(subSession)
	require.NoError(p.T(), err)

	projectName := strings.Split(p.project.ID, ":")[1]
	createdNamespace, err := extnamespaces.CreateNamespace(client, p.downstreamClusterID, projectName, namegen.AppendRandomString("testns-"), "{}", map[string]string{}, map[string]string{})
	require.NoError(p.T(), err)

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
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	client, err := p.client.WithSession(subSession)
	require.NoError(p.T(), err)

	// Test that user can get a specified namespace once granted the permission to do so via roletemplate inheritance bounded
	// by a CRTB.

	projectName := strings.Split(p.project.ID, ":")[1]
	ns, err := extnamespaces.CreateNamespace(client, p.downstreamClusterID, projectName, namegen.AppendRandomString("testns-"), "{}", map[string]string{}, map[string]string{})
	require.NoError(p.T(), err)

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

	anotherNS, err := extnamespaces.CreateNamespace(client, p.downstreamClusterID, projectName, namegen.AppendRandomString("testns-"), "{}", map[string]string{}, map[string]string{})
	require.NoError(p.T(), err)

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

func (p *RTBTestSuite) TestPermissionsCanBeRemoved() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	client, err := p.client.WithSession(subSession)
	require.NoError(p.T(), err)

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
		projectName := strings.Split(project.ID, ":")[1]
		ns, err := extnamespaces.CreateNamespace(client, p.downstreamClusterID, projectName, namegen.AppendRandomString("testns-"), "{}", map[string]string{}, map[string]string{})
		require.NoError(p.T(), err)
		return ns
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

func (p *RTBTestSuite) TestProjectOwnerPermissions() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	client, err := p.client.WithSession(subSession)
	require.NoError(p.T(), err)

	// Grant user the cluster-member role on the local cluster.
	localCluster, err := client.Management.Cluster.ByID(p.downstreamClusterID)
	require.NoError(p.T(), err)

	err = users.AddClusterRoleToUser(client, localCluster, p.testUser, "cluster-member", nil)
	require.NoError(p.T(), err)

	testUser, err := client.AsUser(p.testUser)
	require.NoError(p.T(), err)

	// User creates a project, retrying until RBAC propagates.
	var project *management.Project
	require.Eventually(p.T(), func() bool {
		project, err = testUser.Management.Project.Create(&management.Project{
			ClusterID: p.downstreamClusterID,
			Name:      namegen.AppendRandomString("test-proj-"),
		})
		return err == nil
	}, 2*time.Minute, 2*time.Second, "waiting for user to be able to create a project")

	// Wait for project to become active.
	require.Eventually(p.T(), func() bool {
		proj, err := testUser.Management.Project.ByID(project.ID)
		return err == nil && proj.State == "active"
	}, 2*time.Minute, 2*time.Second, "waiting for project to become active")

	// Wait until user can create namespaces.
	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:     "create",
			Resource: "namespaces",
			Group:    "",
		},
	})
	require.NoError(p.T(), err)

	// User creates a namespace in the project.
	projectName := strings.Split(project.ID, ":")[1]
	ns, err := extnamespaces.CreateNamespace(testUser, p.downstreamClusterID, projectName, namegen.AppendRandomString("testns-"), "{}", map[string]string{}, map[string]string{})
	require.NoError(p.T(), err)

	// Verify user can list pods in the namespace (proves basic access).
	dynamicClient, err := testUser.GetDownStreamClusterClient(p.downstreamClusterID)
	require.NoError(p.T(), err)

	podGVR := corev1.SchemeGroupVersion.WithResource("pods")
	_, err = dynamicClient.Resource(podGVR).Namespace(ns.Name).List(context.TODO(), metav1.ListOptions{})
	require.NoError(p.T(), err)

	// Verify user has both 'project-owner' and 'admin' role bindings in the namespace.
	rbs, err := extrbac.ListRoleBindings(client, p.downstreamClusterID, ns.Name, metav1.ListOptions{})
	require.NoError(p.T(), err)

	rbRoles := map[string]bool{}
	for _, rb := range rbs.Items {
		for _, subject := range rb.Subjects {
			if subject.Name == p.testUser.ID {
				rbRoles[rb.RoleRef.Name] = true
			}
		}
	}
	require.True(p.T(), rbRoles["project-owner"], "expected project-owner role binding for user")
	require.True(p.T(), rbRoles["admin"], "expected admin role binding for user")

	// Verify user can create deployments (extensions group) in the namespace.
	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:      "create",
			Resource:  "deployments",
			Group:     "extensions",
			Namespace: ns.Name,
		},
	})
	require.NoError(p.T(), err)

	// Verify user can list pods.metrics.k8s.io in the namespace.
	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:      "list",
			Resource:  "pods",
			Group:     "metrics.k8s.io",
			Namespace: ns.Name,
		},
	})
	require.NoError(p.T(), err)

	err = users.RemoveClusterRoleFromUser(client, p.testUser)
	require.NoError(p.T(), err)
}

func (p *RTBTestSuite) TestAPIGroupInRoleTemplate() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	client, err := p.client.WithSession(subSession)
	require.NoError(p.T(), err)

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

func (p *RTBTestSuite) TestRemovingUserFromCluster() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	client, err := p.client.WithSession(subSession)
	require.NoError(p.T(), err)

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
	}, 2*time.Minute, 2*time.Second, "failed waiting for clusterRoleBinding to get created")

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

	// User should no longer see any clusters.
	require.Eventually(p.T(), func() bool {
		clusters, err := testUser.Management.Cluster.List(nil)
		return err == nil && len(clusters.Data) == 0
	}, 2*time.Minute, 2*time.Second, "failed revoking cluster access from user")

	// Accessing the cluster by ID should fail with 403.
	_, err = testUser.Management.Cluster.ByID(p.downstreamClusterID)
	require.Error(p.T(), err)
	require.Contains(p.T(), err.Error(), "403")
}

func (p *RTBTestSuite) TestUpgradedSetupRemovingUserFromCluster() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	client, err := p.client.WithSession(subSession)
	require.NoError(p.T(), err)

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
	patchPayload, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
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

	// User should no longer see any clusters.
	require.Eventually(p.T(), func() bool {
		clusters, err := testUser.Management.Cluster.List(nil)
		return err == nil && len(clusters.Data) == 0
	}, 2*time.Minute, 2*time.Second, "failed revoking cluster access from user")

	// Accessing the cluster by ID should fail with 403.
	_, err = testUser.Management.Cluster.ByID(p.downstreamClusterID)
	require.Error(p.T(), err)
	require.Contains(p.T(), err.Error(), "403")
}

func (p *RTBTestSuite) TestUserRolePermissions() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	client, err := p.client.WithSession(subSession)
	require.NoError(p.T(), err)

	enabled := true

	// Create user1 with the standard "user" global role.
	user1Pass := password.GenerateUserPassword("testpass-")
	user1, err := users.CreateUserWithRole(client, &management.User{
		Username: namegen.AppendRandomString("testuser1-"),
		Password: user1Pass,
		Name:     "testuser1",
		Enabled:  &enabled,
	}, "user")
	require.NoError(p.T(), err)
	user1.Password = user1Pass

	// Create user2 with the "user-base" global role.
	user2Pass := password.GenerateUserPassword("testpass-")
	user2, err := users.CreateUserWithRole(client, &management.User{
		Username: namegen.AppendRandomString("testuser2-"),
		Password: user2Pass,
		Name:     "testuser2",
		Enabled:  &enabled,
	}, "user-base")
	require.NoError(p.T(), err)
	user2.Password = user2Pass

	// Create 2 more users (just to pad the user count).
	for i := 0; i < 2; i++ {
		pw := password.GenerateUserPassword("testpass-")
		_, err := users.CreateUserWithRole(client, &management.User{
			Username: namegen.AppendRandomString("testuser-"),
			Password: pw,
			Name:     "testuser",
			Enabled:  &enabled,
		}, "user")
		require.NoError(p.T(), err)
	}

	// Admin should see at least 5 users.
	adminUsers, err := client.Management.User.List(nil)
	require.NoError(p.T(), err)
	require.GreaterOrEqual(p.T(), len(adminUsers.Data), 5)

	user1Client, err := client.AsUser(user1)
	require.NoError(p.T(), err)

	user2Client, err := client.AsUser(user2)
	require.NoError(p.T(), err)

	// user1 (standard "user" role) should only see themselves.
	user1Users, err := user1Client.Management.User.List(nil)
	require.NoError(p.T(), err)
	require.Len(p.T(), user1Users.Data, 1, "user should only see themselves")

	// user1 can see all roleTemplates.
	user1RTs, err := user1Client.Management.RoleTemplate.List(nil)
	require.NoError(p.T(), err)
	require.NotEmpty(p.T(), user1RTs.Data, "user should be able to see all roleTemplates")

	// user2 (user-base role) should only see themselves.
	user2Users, err := user2Client.Management.User.List(nil)
	require.NoError(p.T(), err)
	require.Len(p.T(), user2Users.Data, 1, "user should only see themselves")

	// user2 should not see any role templates.
	user2RTs, err := user2Client.Management.RoleTemplate.List(nil)
	require.NoError(p.T(), err)
	require.Empty(p.T(), user2RTs.Data, "user2 does not have permission to view roleTemplates")
}

// checkAccessAllowed performs a single SelfSubjectAccessReview and returns whether access is allowed.
func checkAccessAllowed(client *rancher.Client, clusterID string, attr *authzv1.ResourceAttributes) (bool, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return false, err
	}

	ssar := &authzv1.SelfSubjectAccessReview{
		Spec: authzv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: attr,
		},
	}

	ssarGVR := authzv1.SchemeGroupVersion.WithResource("selfsubjectaccessreviews")
	resp, err := dynamicClient.Resource(ssarGVR).Create(context.TODO(), extunstructured.MustToUnstructured(ssar), metav1.CreateOptions{})
	if err != nil {
		return false, err
	}

	result := &authzv1.SelfSubjectAccessReview{}
	if err := scheme.Scheme.Convert(resp, result, resp.GroupVersionKind()); err != nil {
		return false, err
	}

	return result.Status.Allowed, nil
}

func (p *RTBTestSuite) TestImpersonationPassthrough() {
	subSession := p.session.NewSession()
	defer subSession.Cleanup()

	client, err := p.client.WithSession(subSession)
	require.NoError(p.T(), err)

	enabled := true

	// Create user1 with standard "user" role.
	user1Pass := password.GenerateUserPassword("testpass-")
	user1, err := users.CreateUserWithRole(client, &management.User{
		Username: namegen.AppendRandomString("imp-user1-"),
		Password: user1Pass,
		Name:     "imp-user1",
		Enabled:  &enabled,
	}, "user")
	require.NoError(p.T(), err)
	user1.Password = user1Pass

	// Create user2 with standard "user" role.
	user2Pass := password.GenerateUserPassword("testpass-")
	user2, err := users.CreateUserWithRole(client, &management.User{
		Username: namegen.AppendRandomString("imp-user2-"),
		Password: user2Pass,
		Name:     "imp-user2",
		Enabled:  &enabled,
	}, "user")
	require.NoError(p.T(), err)
	user2.Password = user2Pass

	localCluster, err := client.Management.Cluster.ByID(p.downstreamClusterID)
	require.NoError(p.T(), err)

	// Give user1 cluster-member and user2 cluster-owner.
	err = users.AddClusterRoleToUser(client, localCluster, user1, "cluster-member", nil)
	require.NoError(p.T(), err)

	err = users.AddClusterRoleToUser(client, localCluster, user2, "cluster-owner", nil)
	require.NoError(p.T(), err)

	user1Client, err := client.AsUser(user1)
	require.NoError(p.T(), err)

	user2Client, err := client.AsUser(user2)
	require.NoError(p.T(), err)

	impersonateAttr := &authzv1.ResourceAttributes{
		Verb:     "impersonate",
		Resource: "users",
		Group:    "",
	}

	// Admin can always impersonate.
	err = extauthz.WaitForAllowed(client, p.downstreamClusterID, []*authzv1.ResourceAttributes{impersonateAttr})
	require.NoError(p.T(), err)

	// User1 is a cluster-member which does not grant impersonate.
	allowed, err := checkAccessAllowed(user1Client, p.downstreamClusterID, impersonateAttr)
	require.NoError(p.T(), err)
	require.False(p.T(), allowed, "cluster-member should not be able to impersonate")

	// User2 is a cluster-owner which allows impersonation.
	err = extauthz.WaitForAllowed(user2Client, p.downstreamClusterID, []*authzv1.ResourceAttributes{impersonateAttr})
	require.NoError(p.T(), err)

	// Create a ClusterRole allowing limited impersonation of user2 only.
	dynamicClient, err := client.GetDownStreamClusterClient(p.downstreamClusterID)
	require.NoError(p.T(), err)

	impRoleName := namegen.AppendRandomString("limited-impersonator-")
	impRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: impRoleName},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Resources:     []string{"users"},
				Verbs:         []string{"impersonate"},
				ResourceNames: []string{user2.ID},
			},
		},
	}

	crResource := dynamicClient.Resource(extrbac.ClusterRoleGroupVersionResource)
	_, err = crResource.Create(context.TODO(), extunstructured.MustToUnstructured(impRole), metav1.CreateOptions{})
	require.NoError(p.T(), err)

	// Create a ClusterRoleBinding binding user1 to the impersonation role.
	impBindingName := namegen.AppendRandomString("limited-impersonator-binding-")
	impBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: impBindingName},
		Subjects: []rbacv1.Subject{
			{
				Kind: "User",
				Name: user1.ID,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     impRoleName,
		},
	}

	crbResource := dynamicClient.Resource(extrbac.ClusterRoleBindingGroupVersionResource)
	_, err = crbResource.Create(context.TODO(), extunstructured.MustToUnstructured(impBinding), metav1.CreateOptions{})
	require.NoError(p.T(), err)

	// User1 should now be able to impersonate user2 specifically.
	err = extauthz.WaitForAllowed(user1Client, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:     "impersonate",
			Resource: "users",
			Group:    "",
			Name:     user2.ID,
		},
	})
	require.NoError(p.T(), err)
}

func TestRTBTestSuite(t *testing.T) {
	suite.Run(t, new(RTBTestSuite))
}
