package integration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/secrets"

	extnamespaces "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/namespaces"
	extrbac "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/rbac"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extauthz "github.com/rancher/shepherd/extensions/kubeapi/authorization"
	"github.com/rancher/shepherd/extensions/users"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/stretchr/testify/require"
	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func (p *RTBTestSuite) TestProjectCreatorGetsOwnerBindings() {
	client, cleanup := p.newSubSession()
	defer cleanup()

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
	ns := p.createNamespace(testUser, p.projectName(project))

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

func (p *RTBTestSuite) TestReadOnlyCannotEditSecret() {
	client, cleanup := p.newSubSession()
	defer cleanup()

	// Create a PRTB giving the test user read-only access to the project.
	_, err := client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		UserID:         p.testUser.ID,
		RoleTemplateID: "read-only",
		ProjectID:      p.project.ID,
	})
	require.NoError(p.T(), err)

	// Create a namespace in the project for testing namespaced secrets.
	ns := p.createNamespace(client, p.projectName(p.project))

	testUser, err := client.AsUser(p.testUser)
	require.NoError(p.T(), err)

	// Read-only user should fail to create a secret.
	_, err = secrets.CreateSecretForCluster(testUser, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "test-secret-"},
		StringData: map[string]string{"abc": "123"},
	}, p.downstreamClusterID, ns.Name)
	require.Error(p.T(), err)
	require.True(p.T(), apierrors.IsForbidden(err), "expected forbidden, got: %v", err)

	// Admin creates a secret so the read-only user can see it but not update it.
	adminSecret, err := secrets.CreateSecretForCluster(client, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "test-secret-"},
		StringData: map[string]string{"abc": "123"},
	}, p.downstreamClusterID, ns.Name)
	require.NoError(p.T(), err)

	// Read-only user should fail to update the secret.
	_, err = secrets.PatchSecret(testUser, p.downstreamClusterID, adminSecret.Name, ns.Name,
		k8stypes.JSONPatchType, secrets.ReplacePatchOP, "/data/abc", "ZmdoCg==", metav1.PatchOptions{})
	require.Error(p.T(), err)
	require.True(p.T(), apierrors.IsForbidden(err), "expected forbidden, got: %v", err)
}

func (p *RTBTestSuite) TestReadOnlyCannotMoveNamespace() {
	client, cleanup := p.newSubSession()
	defer cleanup()

	// Create two projects.
	p1, err := client.Management.Project.Create(&management.Project{
		ClusterID: p.downstreamClusterID,
		Name:      namegen.AppendRandomString("test-proj-"),
	})
	require.NoError(p.T(), err)

	p2, err := client.Management.Project.Create(&management.Project{
		ClusterID: p.downstreamClusterID,
		Name:      namegen.AppendRandomString("test-proj-"),
	})
	require.NoError(p.T(), err)

	// Wait for project namespaces to exist.
	p1Name := strings.ReplaceAll(p1.ID, ":", "-")
	p2Name := strings.ReplaceAll(p2.ID, ":", "-")

	require.Eventually(p.T(), func() bool {
		_, err1 := extnamespaces.GetNamespaceByName(client, p.downstreamClusterID, p1Name)
		_, err2 := extnamespaces.GetNamespaceByName(client, p.downstreamClusterID, p2Name)
		return err1 == nil && err2 == nil
	}, 2*time.Minute, 2*time.Second, fmt.Sprintf("waiting for project namespaces %s and %s to exist", p1Name, p2Name))

	// Give the test user read-only access to both projects.
	_, err = client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		UserID:         p.testUser.ID,
		RoleTemplateID: "read-only",
		ProjectID:      p1.ID,
	})
	require.NoError(p.T(), err)

	_, err = client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		UserID:         p.testUser.ID,
		RoleTemplateID: "read-only",
		ProjectID:      p2.ID,
	})
	require.NoError(p.T(), err)

	// Create a namespace in project 1.
	ns := p.createNamespace(client, p.projectName(p1))

	testUser, err := client.AsUser(p.testUser)
	require.NoError(p.T(), err)

	// Wait until the read-only user can see the namespace.
	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:     "get",
			Resource: "namespaces",
			Name:     ns.Name,
		},
	})
	require.NoError(p.T(), err)

	// Read-only user should fail to move the namespace to project 2 by updating the projectId annotation.
	dynamicClient, err := testUser.GetDownStreamClusterClient(p.downstreamClusterID)
	require.NoError(p.T(), err)

	nsGVR := corev1.SchemeGroupVersion.WithResource("namespaces")
	patchPayload := fmt.Sprintf(`{"metadata":{"annotations":{"field.cattle.io/projectId":"%s:%s"}}}`, p.downstreamClusterID, p.projectName(p2))
	_, err = dynamicClient.Resource(nsGVR).Patch(context.TODO(), ns.Name, k8stypes.MergePatchType, []byte(patchPayload), metav1.PatchOptions{})
	require.Error(p.T(), err)
	require.True(p.T(), apierrors.IsForbidden(err), "expected forbidden, got: %v", err)
}
