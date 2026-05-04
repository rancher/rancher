package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/secrets"
	"github.com/rancher/shepherd/clients/rancher"

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

	var systemProject management.Project
	found := false
	for _, project := range projects.Data {
		if project.Name == "System" {
			systemProject = project
			found = true
			break
		}
	}
	p.Require().True(found, "System project not found")

	// Attempting to delete the System project should return 405.
	err = client.Management.Project.Delete(&systemProject)
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

	saList, err := dynamicClient.Resource(saGVR).Namespace("").List(context.TODO(), metav1.ListOptions{
		FieldSelector: "metadata.name=default",
	})
	p.Require().NoError(err)

	for _, sa := range saList.Items {
		ns := sa.GetNamespace()
		if _, ok := systemNamespaces[ns]; !ok || ns == "kube-system" {
			continue
		}
		automount, found, _ := unstructured.NestedBool(sa.Object, "automountServiceAccountToken")
		p.Require().True(found, fmt.Sprintf("automountServiceAccountToken not found for service account %s in namespace %s", sa.GetName(), ns))
		p.Require().False(automount, fmt.Sprintf("automountServiceAccountToken should be false for service account %s in namespace %s", sa.GetName(), ns))
	}
}

// TestProjectResourceQuotaFields tests that creating a project with a resource quota
// and namespace default resource quota correctly stores those fields.
func (p *RTBTestSuite) TestProjectResourceQuotaFields() {
	client := p.newSubSession()

	pq := &management.ProjectResourceQuota{
		Limit: &management.ResourceQuotaLimit{Pods: "100"},
	}
	nsq := &management.NamespaceResourceQuota{
		Limit: &management.ResourceQuotaLimit{Pods: "100"},
	}

	project, err := client.Management.Project.Create(&management.Project{
		Name:                          namegen.AppendRandomString("test-"),
		ClusterID:                     p.downstreamClusterID,
		ResourceQuota:                 pq,
		NamespaceDefaultResourceQuota: nsq,
	})
	p.Require().NoError(err)

	p.Require().NotNil(project.ResourceQuota)
	p.Require().Equal("100", project.ResourceQuota.Limit.Pods)
	p.Require().NotNil(project.NamespaceDefaultResourceQuota)
	p.Require().Equal("100", project.NamespaceDefaultResourceQuota.Limit.Pods)
}

// TestProjectQuotaAPIValidation tests project-level resource quota API validation:
// - namespaceDefaultResourceQuota must be provided when resourceQuota is set
// - resourceQuota must be provided when namespaceDefaultResourceQuota is set
// - namespace default quota fields must not exceed project quota
// - namespace default quota must have all fields defined on the project quota
func (p *RTBTestSuite) TestProjectQuotaAPIValidation() {
	client := p.newSubSession()

	pq := &management.ProjectResourceQuota{
		Limit: &management.ResourceQuotaLimit{Pods: "100"},
	}
	nsqLarge := &management.NamespaceResourceQuota{
		Limit: &management.ResourceQuotaLimit{Pods: "200"},
	}

	var apiErr *clientbase.APIError

	// resourceQuota without namespaceDefaultResourceQuota should fail (422).
	_, err := client.Management.Project.Create(&management.Project{
		Name:          namegen.AppendRandomString("test-"),
		ClusterID:     p.downstreamClusterID,
		ResourceQuota: pq,
	})
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)

	// namespaceDefaultResourceQuota without resourceQuota should fail (422).
	_, err = client.Management.Project.Create(&management.Project{
		Name:      namegen.AppendRandomString("test-"),
		ClusterID: p.downstreamClusterID,
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "100"},
		},
	})
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)

	// Namespace default quota exceeding project quota should fail (422).
	_, err = client.Management.Project.Create(&management.Project{
		Name:                          namegen.AppendRandomString("test-"),
		ClusterID:                     p.downstreamClusterID,
		ResourceQuota:                 pq,
		NamespaceDefaultResourceQuota: nsqLarge,
	})
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)

	// Namespace default quota missing fields defined on project quota should fail (422).
	pqMulti := &management.ProjectResourceQuota{
		Limit: &management.ResourceQuotaLimit{Pods: "100", Services: "100"},
	}
	nsqIncomplete := &management.NamespaceResourceQuota{
		Limit: &management.ResourceQuotaLimit{Pods: "100"},
	}

	proj, err := client.Management.Project.Create(&management.Project{
		Name:      namegen.AppendRandomString("test-"),
		ClusterID: p.downstreamClusterID,
	})
	p.Require().NoError(err)

	_, err = client.Management.Project.Update(proj, map[string]any{
		"resourceQuota":                 pqMulti,
		"namespaceDefaultResourceQuota": nsqIncomplete,
	})
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)
}

// TestProjectContainerDefaultResourceLimit tests that creating a project with a
// containerDefaultResourceLimit correctly stores the limit, and that it can be cleared.
func (p *RTBTestSuite) TestProjectContainerDefaultResourceLimit() {
	client := p.newSubSession()

	pq := &management.ProjectResourceQuota{
		Limit: &management.ResourceQuotaLimit{Pods: "100"},
	}
	nsq := &management.NamespaceResourceQuota{
		Limit: &management.ResourceQuotaLimit{Pods: "100"},
	}
	lmt := &management.ContainerResourceLimit{
		RequestsCPU:    "1",
		RequestsMemory: "1Gi",
		LimitsCPU:      "2",
		LimitsMemory:   "2Gi",
	}

	project, err := client.Management.Project.Create(&management.Project{
		Name:                          namegen.AppendRandomString("test-"),
		ClusterID:                     p.downstreamClusterID,
		ResourceQuota:                 pq,
		NamespaceDefaultResourceQuota: nsq,
		ContainerDefaultResourceLimit: lmt,
	})
	p.Require().NoError(err)
	p.Require().NotNil(project.ResourceQuota)
	p.Require().NotNil(project.ContainerDefaultResourceLimit)

	// Clear the container limit.
	updated, err := client.Management.Project.Update(project, map[string]any{
		"containerDefaultResourceLimit": nil,
	})
	p.Require().NoError(err)
	p.Require().Nil(updated.ContainerDefaultResourceLimit)
}

// createNamespaceWithQuota creates a namespace in the given project with an optional
// resource quota annotation. If quota is nil, no resource quota annotation is set and
// the project's namespaceDefaultResourceQuota will apply.
func (p *RTBTestSuite) createNamespaceWithQuota(client *rancher.Client, projName string, quota map[string]string) *corev1.Namespace {
	annotations := map[string]string{}
	if quota != nil {
		q := map[string]any{"limit": quota}
		b, err := json.Marshal(q)
		p.Require().NoError(err)
		annotations["field.cattle.io/resourceQuota"] = string(b)
	}
	ns, err := extnamespaces.CreateNamespace(client, p.downstreamClusterID, projName, namegen.AppendRandomString("testns-"), "{}", map[string]string{}, annotations)
	p.Require().NoError(err)
	return ns
}

// waitForResourceQuota waits for the Rancher resource quota controller to create a k8s
// ResourceQuota object in the given namespace (identified by the default-resource-quota label)
// and returns its spec.hard as a string map.
func (p *RTBTestSuite) waitForResourceQuota(client *rancher.Client, nsName string) map[string]string {
	dynamicClient, err := client.GetDownStreamClusterClient(p.downstreamClusterID)
	p.Require().NoError(err)

	rqGVR := corev1.SchemeGroupVersion.WithResource("resourcequotas")

	var hard map[string]string
	p.Require().Eventually(func() bool {
		rqList, err := dynamicClient.Resource(rqGVR).Namespace(nsName).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "resourcequota.management.cattle.io/default-resource-quota=true",
		})
		if err != nil || len(rqList.Items) == 0 {
			return false
		}
		specRaw, found, _ := unstructured.NestedMap(rqList.Items[0].Object, "spec", "hard")
		if !found {
			return false
		}
		hard = make(map[string]string, len(specRaw))
		for k, v := range specRaw {
			hard[k] = fmt.Sprintf("%v", v)
		}
		return len(hard) > 0
	}, 2*time.Minute, 2*time.Second, "waiting for ResourceQuota in namespace %s", nsName)

	return hard
}

// waitForProjectUsedLimit waits for the project's usedLimit for the given field
// to equal the expected value.
func (p *RTBTestSuite) waitForProjectUsedLimit(client *rancher.Client, projectID, field, value string) {
	p.Require().Eventually(func() bool {
		proj, err := client.Management.Project.ByID(projectID)
		if err != nil || proj.ResourceQuota == nil || proj.ResourceQuota.UsedLimit == nil {
			return false
		}
		switch field {
		case "pods":
			return proj.ResourceQuota.UsedLimit.Pods == value
		case "services":
			return proj.ResourceQuota.UsedLimit.Services == value
		}
		return false
	}, 2*time.Minute, 2*time.Second, "waiting for project usedLimit.%s=%s", field, value)
}

// TestNamespaceResourceQuotaCreated tests that when a namespace is created in a project
// with an explicit resource quota annotation, the Rancher controller creates a k8s
// ResourceQuota object in the namespace with the requested limits.
func (p *RTBTestSuite) TestNamespaceResourceQuotaCreated() {
	client := p.newSubSession()

	project, err := client.Management.Project.Create(&management.Project{
		Name:      namegen.AppendRandomString("test-"),
		ClusterID: p.downstreamClusterID,
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "100"},
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "100"},
		},
	})
	p.Require().NoError(err)

	ns := p.createNamespaceWithQuota(client, p.projectName(project), map[string]string{"pods": "4"})

	hard := p.waitForResourceQuota(client, ns.Name)
	p.Require().Equal("4", hard["pods"])
}

// TestNamespaceDefaultQuotaApplied tests that when a namespace is created in a project
// without an explicit quota annotation, the project's namespaceDefaultResourceQuota is
// applied by the controller.
func (p *RTBTestSuite) TestNamespaceDefaultQuotaApplied() {
	client := p.newSubSession()

	project, err := client.Management.Project.Create(&management.Project{
		Name:      namegen.AppendRandomString("test-"),
		ClusterID: p.downstreamClusterID,
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "100"},
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "4"},
		},
	})
	p.Require().NoError(err)

	// Create namespace without explicit quota — should get the project default.
	ns := p.createNamespaceWithQuota(client, p.projectName(project), nil)

	hard := p.waitForResourceQuota(client, ns.Name)
	p.Require().Equal("4", hard["pods"])
}

// TestProjectUsedQuotaUpdated tests that the project's usedLimit is updated by the
// controller when namespaces with quotas are created in the project.
func (p *RTBTestSuite) TestProjectUsedQuotaUpdated() {
	client := p.newSubSession()

	project, err := client.Management.Project.Create(&management.Project{
		Name:      namegen.AppendRandomString("test-"),
		ClusterID: p.downstreamClusterID,
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "100"},
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "4"},
		},
	})
	p.Require().NoError(err)

	// Create namespace — default quota of 4 pods should apply.
	ns := p.createNamespaceWithQuota(client, p.projectName(project), nil)
	p.waitForResourceQuota(client, ns.Name)

	p.waitForProjectUsedLimit(client, project.ID, "pods", "4")
}

// TestProjectQuotaUpdateAppliedToNamespace tests that when a project is updated to
// add a resource quota, existing namespaces in the project get a ResourceQuota created.
func (p *RTBTestSuite) TestProjectQuotaUpdateAppliedToNamespace() {
	client := p.newSubSession()

	// Create project without quota.
	project, err := client.Management.Project.Create(&management.Project{
		Name:      namegen.AppendRandomString("test-"),
		ClusterID: p.downstreamClusterID,
	})
	p.Require().NoError(err)

	// Create a namespace (no quota yet on the project).
	ns := p.createNamespaceWithQuota(client, p.projectName(project), nil)

	// Update the project to add quota.
	_, err = client.Management.Project.Update(project, map[string]any{
		"resourceQuota": &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "100"},
		},
		"namespaceDefaultResourceQuota": &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "4"},
		},
	})
	p.Require().NoError(err)

	// The controller should apply the default quota to the existing namespace.
	hard := p.waitForResourceQuota(client, ns.Name)
	p.Require().Equal("4", hard["pods"])
}

// TestProjectUsedQuotaExactMatch tests that when all of a project's quota is consumed
// by namespaces, the project quota cannot be reduced below the used amount.
func (p *RTBTestSuite) TestProjectUsedQuotaExactMatch() {
	client := p.newSubSession()

	project, err := client.Management.Project.Create(&management.Project{
		Name:      namegen.AppendRandomString("test-"),
		ClusterID: p.downstreamClusterID,
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "10"},
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "2"},
		},
	})
	p.Require().NoError(err)

	// Create two namespaces: 2 + 8 = 10 pods (full quota).
	ns1 := p.createNamespaceWithQuota(client, p.projectName(project), map[string]string{"pods": "2"})
	p.waitForResourceQuota(client, ns1.Name)

	ns2 := p.createNamespaceWithQuota(client, p.projectName(project), map[string]string{"pods": "8"})
	p.waitForResourceQuota(client, ns2.Name)

	p.waitForProjectUsedLimit(client, project.ID, "pods", "10")

	// Try reducing the project quota below the used amount — should fail (422).
	var apiErr *clientbase.APIError
	_, err = client.Management.Project.Update(project, map[string]any{
		"resourceQuota": &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "8"},
		},
		"namespaceDefaultResourceQuota": &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "1"},
		},
	})
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)
}

// TestProjectQuotaAddRemoveFields tests adding and removing quota fields on a project
// and verifying that the usedLimit is updated accordingly.
func (p *RTBTestSuite) TestProjectQuotaAddRemoveFields() {
	client := p.newSubSession()

	project, err := client.Management.Project.Create(&management.Project{
		Name:      namegen.AppendRandomString("test-"),
		ClusterID: p.downstreamClusterID,
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "10"},
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "2"},
		},
	})
	p.Require().NoError(err)

	// Create two namespaces using the default quota of 2 pods each.
	ns1 := p.createNamespaceWithQuota(client, p.projectName(project), map[string]string{"pods": "2"})
	p.waitForResourceQuota(client, ns1.Name)
	p.waitForProjectUsedLimit(client, project.ID, "pods", "2")

	ns2 := p.createNamespaceWithQuota(client, p.projectName(project), map[string]string{"pods": "2"})
	p.waitForResourceQuota(client, ns2.Name)
	p.waitForProjectUsedLimit(client, project.ID, "pods", "4")

	// Trying to add services field with a default that exceeds project limit should fail.
	var apiErr *clientbase.APIError
	_, err = client.Management.Project.Update(project, map[string]any{
		"resourceQuota": &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "10", Services: "10"},
		},
		"namespaceDefaultResourceQuota": &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "2", Services: "7"},
		},
	})
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)

	// Add services field with a valid default.
	project, err = client.Management.Project.Update(project, map[string]any{
		"resourceQuota": &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "10", Services: "10"},
		},
		"namespaceDefaultResourceQuota": &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "2", Services: "2"},
		},
	})
	p.Require().NoError(err)

	// Controller should propagate services default to existing namespaces.
	p.waitForProjectUsedLimit(client, project.ID, "services", "4")

	// Remove the services field.
	project, err = client.Management.Project.Update(project, map[string]any{
		"resourceQuota": &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "10"},
		},
		"namespaceDefaultResourceQuota": &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "2"},
		},
	})
	p.Require().NoError(err)

	// After removing services, usedLimit.services should drop to 0.
	p.waitForProjectUsedLimit(client, project.ID, "services", "0")
}

// TestProjectQuotaCannotExceedWithExistingNamespaces tests that setting a project quota
// where default * existing namespace count exceeds the limit is rejected.
func (p *RTBTestSuite) TestProjectQuotaCannotExceedWithExistingNamespaces() {
	client := p.newSubSession()

	// Create project without quota.
	project, err := client.Management.Project.Create(&management.Project{
		Name:      namegen.AppendRandomString("test-"),
		ClusterID: p.downstreamClusterID,
	})
	p.Require().NoError(err)

	// Create 4 namespaces in the project.
	for range 4 {
		p.createNamespaceWithQuota(client, p.projectName(project), nil)
	}

	// Try setting quota where default (2) * 4 namespaces = 8 > limit (5) — should fail.
	var apiErr *clientbase.APIError
	_, err = client.Management.Project.Update(project, map[string]any{
		"resourceQuota": &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "5"},
		},
		"namespaceDefaultResourceQuota": &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "2"},
		},
	})
	p.Require().True(errors.As(err, &apiErr), "expected APIError, got: %v", err)
	p.Require().Equal(http.StatusUnprocessableEntity, apiErr.StatusCode)
}

// TestNamespaceQuotaExceedsProjectLimit tests that the controller handles a namespace
// whose requested quota exceeds the project limit by zeroing overused resources in the
// created k8s ResourceQuota, and that the project's usedLimit is not inflated.
func (p *RTBTestSuite) TestNamespaceQuotaExceedsProjectLimit() {
	client := p.newSubSession()

	project, err := client.Management.Project.Create(&management.Project{
		Name:      namegen.AppendRandomString("test-"),
		ClusterID: p.downstreamClusterID,
		ResourceQuota: &management.ProjectResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "100"},
		},
		NamespaceDefaultResourceQuota: &management.NamespaceResourceQuota{
			Limit: &management.ResourceQuotaLimit{Pods: "100"},
		},
	})
	p.Require().NoError(err)

	// Create namespace requesting more pods than the project allows.
	ns := p.createNamespaceWithQuota(client, p.projectName(project), map[string]string{"pods": "200"})

	// The controller should still create a ResourceQuota, but with zeroed overused resources.
	hard := p.waitForResourceQuota(client, ns.Name)
	p.Require().Contains(hard, "pods")
	podsVal := hard["pods"]
	p.Require().NotEqual("200", podsVal, "quota should not be set to the overused value")
}
