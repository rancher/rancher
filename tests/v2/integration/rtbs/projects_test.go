package integration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/secrets"

	extnamespaces "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/namespaces"
	extrbac "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/rbac"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extauthz "github.com/rancher/shepherd/extensions/kubeapi/authorization"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/clientbase"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	authzv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func (p *RTBTestSuite) TestProjectCreatorGetsOwnerBindings() {
	client := p.newSubSession()

	// Grant user the cluster-member role on the local cluster.
	localCluster, err := client.Management.Cluster.ByID(p.downstreamClusterID)
	p.Require().NoError(err)

	err = users.AddClusterRoleToUser(client, localCluster, p.testUser, "cluster-member", nil)
	p.Require().NoError(err)

	p.T().Cleanup(func() {
		err := users.RemoveClusterRoleFromUser(client, p.testUser)
		p.Require().NoError(err)
	})

	testUser, err := client.AsUser(p.testUser)
	p.Require().NoError(err)

	// User creates a project, retrying until RBAC propagates.
	var project *management.Project
	p.Require().Eventually(func() bool {
		project, err = testUser.Management.Project.Create(&management.Project{
			ClusterID: p.downstreamClusterID,
			Name:      namegen.AppendRandomString("test-proj-"),
		})
		return err == nil
	}, 2*time.Minute, 2*time.Second, "waiting for user to be able to create a project")

	// Wait for project to become active.
	p.Require().Eventually(func() bool {
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
	p.Require().NoError(err)

	// User creates a namespace in the project.
	ns := p.createNamespace(testUser, p.projectName(project))

	// Verify user can list pods in the namespace (proves basic access).
	dynamicClient, err := testUser.GetDownStreamClusterClient(p.downstreamClusterID)
	p.Require().NoError(err)

	podGVR := corev1.SchemeGroupVersion.WithResource("pods")
	_, err = dynamicClient.Resource(podGVR).Namespace(ns.Name).List(context.TODO(), metav1.ListOptions{})
	p.Require().NoError(err)

	// Verify user has both 'project-owner' and 'admin' role bindings in the namespace.
	rbs, err := extrbac.ListRoleBindings(client, p.downstreamClusterID, ns.Name, metav1.ListOptions{})
	p.Require().NoError(err)

	rbRoles := []string{}
	for _, rb := range rbs.Items {
		for _, subject := range rb.Subjects {
			if subject.Name == p.testUser.ID {
				rbRoles = append(rbRoles, rb.RoleRef.Name)
			}
		}
	}

	p.Require().Contains(rbRoles, "project-owner", "expected project-owner role binding for user")
	p.Require().Contains(rbRoles, "admin", "expected admin role binding for user")

	// Verify user can create deployments (extensions group) in the namespace.
	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:      "create",
			Resource:  "deployments",
			Group:     "extensions",
			Namespace: ns.Name,
		},
	})
	p.Require().NoError(err)

	// Verify user can list pods.metrics.k8s.io in the namespace.
	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:      "list",
			Resource:  "pods",
			Group:     "metrics.k8s.io",
			Namespace: ns.Name,
		},
	})
	p.Require().NoError(err)
}

func (p *RTBTestSuite) TestReadOnlyCannotEditSecret() {
	client := p.newSubSession()

	// Create a PRTB giving the test user read-only access to the project.
	_, err := client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		UserID:         p.testUser.ID,
		RoleTemplateID: "read-only",
		ProjectID:      p.project.ID,
	})
	p.Require().NoError(err)

	// Create a namespace in the project for testing namespaced secrets.
	ns := p.createNamespace(client, p.projectName(p.project))

	testUser, err := client.AsUser(p.testUser)
	p.Require().NoError(err)

	// Read-only user should fail to create a secret.
	_, err = secrets.CreateSecretForCluster(testUser, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "test-secret-"},
		StringData: map[string]string{"abc": "123"},
	}, p.downstreamClusterID, ns.Name)
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden, got: %v", err)

	// Admin creates a secret so the read-only user can see it but not update it.
	adminSecret, err := secrets.CreateSecretForCluster(client, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "test-secret-"},
		StringData: map[string]string{"abc": "123"},
	}, p.downstreamClusterID, ns.Name)
	p.Require().NoError(err)

	// Read-only user should fail to update the secret.
	_, err = secrets.PatchSecret(testUser, p.downstreamClusterID, adminSecret.Name, ns.Name,
		k8stypes.JSONPatchType, secrets.ReplacePatchOP, "/data/abc", "ZmdoCg==", metav1.PatchOptions{})
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden, got: %v", err)
}

func (p *RTBTestSuite) TestReadOnlyCannotMoveNamespace() {
	client := p.newSubSession()

	// Create two projects.
	p1, err := client.Management.Project.Create(&management.Project{
		ClusterID: p.downstreamClusterID,
		Name:      namegen.AppendRandomString("test-proj-"),
	})
	p.Require().NoError(err)

	p2, err := client.Management.Project.Create(&management.Project{
		ClusterID: p.downstreamClusterID,
		Name:      namegen.AppendRandomString("test-proj-"),
	})
	p.Require().NoError(err)

	// Wait for project namespaces to exist.
	p1Name := strings.ReplaceAll(p1.ID, ":", "-")
	p2Name := strings.ReplaceAll(p2.ID, ":", "-")

	p.Require().Eventually(func() bool {
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
	p.Require().NoError(err)

	_, err = client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		UserID:         p.testUser.ID,
		RoleTemplateID: "read-only",
		ProjectID:      p2.ID,
	})
	p.Require().NoError(err)

	// Create a namespace in project 1.
	ns := p.createNamespace(client, p.projectName(p1))

	testUser, err := client.AsUser(p.testUser)
	p.Require().NoError(err)

	// Wait until the read-only user can see the namespace.
	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:     "get",
			Resource: "namespaces",
			Name:     ns.Name,
		},
	})
	p.Require().NoError(err)

	// Read-only user should fail to move the namespace to project 2 by updating the projectId annotation.
	dynamicClient, err := testUser.GetDownStreamClusterClient(p.downstreamClusterID)
	p.Require().NoError(err)

	nsGVR := corev1.SchemeGroupVersion.WithResource("namespaces")
	patchPayload := fmt.Sprintf(`{"metadata":{"annotations":{"field.cattle.io/projectId":"%s:%s"}}}`, p.downstreamClusterID, p.projectName(p2))
	_, err = dynamicClient.Resource(nsGVR).Patch(context.TODO(), ns.Name, k8stypes.MergePatchType, []byte(patchPayload), metav1.PatchOptions{})
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden, got: %v", err)
}

func (p *RTBTestSuite) TestSystemProjectCreated() {
	client := p.newSubSession()

	projects, err := client.Management.Project.List(&types.ListOpts{
		Filters: map[string]any{
			"clusterId": p.downstreamClusterID,
		},
	})
	p.Require().NoError(err)

	systemProjectLabel := "authz.management.cattle.io/system-project"
	defaultProjectLabel := "authz.management.cattle.io/default-project"

	initialProjects := map[string]string{
		"Default": defaultProjectLabel,
		"System":  systemProjectLabel,
	}

	var requiredProjects []string
	for _, project := range projects.Data {
		if label, ok := initialProjects[project.Name]; ok {
			p.Require().Equal("true", project.Labels[label])
			requiredProjects = append(requiredProjects, project.Name)
		}
	}

	p.Require().Len(requiredProjects, len(initialProjects))
}

func (p *RTBTestSuite) TestSystemProjectCannotBeDeleted() {
	client := p.newSubSession()

	projects, err := client.Management.Project.List(&types.ListOpts{
		Filters: map[string]any{
			"clusterId": p.downstreamClusterID,
		},
	})
	p.Require().NoError(err)

	var systemProject *management.Project
	for _, project := range projects.Data {
		if project.Name == "System" {
			systemProject = &project
			break
		}
	}
	p.Require().NotNil(systemProject, "System project not found")

	// Attempting to delete the System project should return 405.
	err = client.Management.Project.Delete(systemProject)
	p.Require().Error(err)

	var apiErr *clientbase.APIError
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusMethodNotAllowed, apiErr.StatusCode)
	p.Require().Contains(apiErr.Body, "System Project cannot be deleted")
}

func (p *RTBTestSuite) TestSystemNamespacesDefaultServiceAccount() {
	client := p.newSubSession()

	setting, err := client.Management.Setting.ByID("system-namespaces")
	p.Require().NoError(err)

	systemNamespaces := make(map[string]any)
	for ns := range strings.SplitSeq(setting.Value, ",") {
		trimmed := strings.TrimSpace(ns)
		if trimmed != "" {
			systemNamespaces[trimmed] = true
		}
	}

	dynamicClient, err := client.GetDownStreamClusterClient(p.downstreamClusterID)
	p.Require().NoError(err)

	saGVR := corev1.SchemeGroupVersion.WithResource("serviceaccounts")

	for ns := range systemNamespaces {
		if ns == "kube-system" {
			continue
		}

		p.Require().Eventually(func() bool {
			saList, err := dynamicClient.Resource(saGVR).Namespace(ns).List(context.TODO(), metav1.ListOptions{
				FieldSelector: "metadata.name=default",
			})
			if err != nil || len(saList.Items) == 0 {
				return false
			}

			sa := saList.Items[0]
			automount, found, _ := unstructured.NestedBool(sa.Object, "automountServiceAccountToken")
			return found && !automount
		}, 2*time.Minute, 2*time.Second, fmt.Sprintf("default service account in namespace %s does not have automountServiceAccountToken set to false", ns))
	}
}
