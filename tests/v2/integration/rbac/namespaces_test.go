package integration

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	extnamespaces "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/namespaces"
	extrbac "github.com/rancher/rancher/tests/v2/integration/actions/kubeapi/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extauthz "github.com/rancher/shepherd/extensions/kubeapi/authorization"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/stretchr/testify/assert"
	authzv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
)

const projectIDAnnotation = "field.cattle.io/projectId"

// createProject creates a project in the downstream cluster and waits for its backing namespace, which holds the
// project's ProjectRoleTemplateBindings.
func (p *RTBTestSuite) createProject(client *rancher.Client) *management.Project {
	project, err := client.Management.Project.Create(&management.Project{
		ClusterID: p.downstreamClusterID,
		Name:      namegen.AppendRandomString("test-proj-"),
	})
	p.Require().NoError(err)

	backingNamespace := strings.ReplaceAll(project.ID, ":", "-")
	p.Require().Eventually(func() bool {
		_, err := extnamespaces.GetNamespaceByName(client, p.downstreamClusterID, backingNamespace)
		return err == nil
	}, 2*time.Minute, 2*time.Second, "waiting for project namespace %s to exist", backingNamespace)
	return project
}

// createProjectRoleTemplate creates a project RoleTemplate with the given rules and returns its ID.
func (p *RTBTestSuite) createProjectRoleTemplate(client *rancher.Client, rules ...management.PolicyRule) string {
	rt, err := client.Management.RoleTemplate.Create(&management.RoleTemplate{
		Context: "project",
		Name:    namegen.AppendRandomString("test-rt-"),
		Rules:   rules,
	})
	p.Require().NoError(err)
	return rt.ID
}

// createInheritingRoleTemplate creates a project RoleTemplate with no rules of its own that inherits from the given
// RoleTemplates, and returns its ID.
func (p *RTBTestSuite) createInheritingRoleTemplate(client *rancher.Client, roleTemplateIDs ...string) string {
	rt, err := client.Management.RoleTemplate.Create(&management.RoleTemplate{
		Context:         "project",
		Name:            namegen.AppendRandomString("test-rt-"),
		RoleTemplateIDs: roleTemplateIDs,
	})
	p.Require().NoError(err)
	return rt.ID
}

// bindToProject binds the user to the RoleTemplate in the project. The binding is deleted during cleanup, before the
// RoleTemplate is, because Rancher can't finish removing a binding whose RoleTemplate is already gone: the webhook
// rejects the update that removes its finalizers.
func (p *RTBTestSuite) bindToProject(client *rancher.Client, user *management.User, roleTemplateID string, project *management.Project) {
	prtb, err := client.Management.ProjectRoleTemplateBinding.Create(&management.ProjectRoleTemplateBinding{
		UserID:         user.ID,
		RoleTemplateID: roleTemplateID,
		ProjectID:      project.ID,
	})
	p.Require().NoError(err)

	// Cleanup runs in reverse order, so this runs before the cleanup of the RoleTemplate, which was created first.
	client.Session.RegisterCleanupFunc(func() error {
		return deletePRTB(client, prtb)
	})
}

// deletePRTB deletes the binding and waits for it to be removed.
func deletePRTB(client *rancher.Client, prtb *management.ProjectRoleTemplateBinding) error {
	gvr := extrbac.ProjectRoleTemplateBindingGroupVersionResource

	// Deleting the PRTB before its remove-handler finalizer is added orphans its bindings (see rtbFinalizers).
	err := kwait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 2*time.Minute, true, func(context.Context) (bool, error) {
		finalizers, err := rtbFinalizers(client, gvr, prtb.ID)
		return err == nil && slices.Contains(finalizers, prtbRemoveFinalizer), nil
	})
	if err != nil {
		return fmt.Errorf("waiting for the remove-handler finalizer on PRTB %s: %w", prtb.ID, err)
	}

	if err := client.Management.ProjectRoleTemplateBinding.Delete(prtb); err != nil {
		return fmt.Errorf("deleting PRTB %s: %w", prtb.ID, err)
	}

	err = kwait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 2*time.Minute, true, func(context.Context) (bool, error) {
		_, err := rtbFinalizers(client, gvr, prtb.ID)
		return apierrors.IsNotFound(err), nil
	})
	if err != nil {
		return fmt.Errorf("waiting for PRTB %s to be removed: %w", prtb.ID, err)
	}
	return nil
}

// namespaceResource returns a client for namespaces in the downstream cluster that acts as the given user.
func (p *RTBTestSuite) namespaceResource(client *rancher.Client) dynamic.NamespaceableResourceInterface {
	dynamicClient, err := client.GetDownStreamClusterClient(p.downstreamClusterID)
	p.Require().NoError(err)
	return dynamicClient.Resource(extnamespaces.NamespaceGroupVersionResource)
}

// projectAnnotation returns the value of the projectId annotation for a namespace in the project.
func (p *RTBTestSuite) projectAnnotation(project *management.Project) string {
	return p.downstreamClusterID + ":" + p.projectName(project)
}

// createNamespaceAsUser creates a namespace in the project as userClient. If it succeeds, the namespace is deleted by
// client during cleanup, because the user might not be allowed to delete it.
func (p *RTBTestSuite) createNamespaceAsUser(client, userClient *rancher.Client, project *management.Project) (*unstructured.Unstructured, error) {
	ns := &unstructured.Unstructured{}
	ns.SetAPIVersion("v1")
	ns.SetKind("Namespace")
	ns.SetName(namegen.AppendRandomString("testns-"))
	ns.SetAnnotations(map[string]string{projectIDAnnotation: p.projectAnnotation(project)})
	created, err := p.namespaceResource(userClient).Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	adminResource := p.namespaceResource(client)
	client.Session.RegisterCleanupFunc(func() error {
		err := adminResource.Delete(context.TODO(), created.GetName(), metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return p.waitForNamespaceDeleted(client, created.GetName())
	})
	return created, nil
}

// waitForNamespaceDeleted waits for the namespace to be removed. Deleting a namespace that is still terminating fails,
// so this stops later cleanup from tripping over it.
func (p *RTBTestSuite) waitForNamespaceDeleted(client *rancher.Client, name string) error {
	return kwait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 2*time.Minute, true, func(context.Context) (bool, error) {
		_, err := extnamespaces.GetNamespaceByName(client, p.downstreamClusterID, name)
		return apierrors.IsNotFound(err), nil
	})
}

// moveNamespaceWithPatch moves the namespace to the project with a merge patch of its projectId annotation.
func (p *RTBTestSuite) moveNamespaceWithPatch(client *rancher.Client, name string, project *management.Project) (*unstructured.Unstructured, error) {
	patchPayload := fmt.Sprintf(`{"metadata":{"annotations":{%q:%q}}}`, projectIDAnnotation, p.projectAnnotation(project))
	return p.namespaceResource(client).Patch(context.TODO(), name, k8stypes.MergePatchType, []byte(patchPayload), metav1.PatchOptions{})
}

// moveNamespaceWithUpdate moves the namespace to the project by getting it and updating its projectId annotation.
// Update and patch are separate verbs, so this is how a user with only update moves a namespace.
func (p *RTBTestSuite) moveNamespaceWithUpdate(client *rancher.Client, name string, project *management.Project) (*unstructured.Unstructured, error) {
	nsResource := p.namespaceResource(client)
	current, err := nsResource.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	annotations := current.GetAnnotations()
	annotations[projectIDAnnotation] = p.projectAnnotation(project)
	current.SetAnnotations(annotations)
	return nsResource.Update(context.TODO(), current, metav1.UpdateOptions{})
}

// namespaceRule returns a rule granting the given verbs on namespaces.
func namespaceRule(verbs ...string) management.PolicyRule {
	return management.PolicyRule{
		APIGroups: []string{""},
		Resources: []string{"namespaces"},
		Verbs:     verbs,
	}
}

// namespaceAttributes returns the attributes of a request for the verb on a namespace. Kubernetes treats a request on
// a namespace as being in that namespace, so these also match RoleBindings inside it.
func namespaceAttributes(verb, name string) *authzv1.ResourceAttributes {
	return &authzv1.ResourceAttributes{
		Verb:      verb,
		Resource:  "namespaces",
		Namespace: name,
		Name:      name,
	}
}

// manageNamespacesAttributes returns the attributes the webhook checks before a namespace is created in or moved into
// the project.
func manageNamespacesAttributes(projectName string) *authzv1.ResourceAttributes {
	return &authzv1.ResourceAttributes{
		Verb:     "manage-namespaces",
		Group:    "management.cattle.io",
		Resource: "projects",
		Name:     projectName,
	}
}

// TestCreateNamespacesRoleCanOnlyCreateNamespaces tests that a user bound to the create-ns RoleTemplate can create
// namespaces in the project, but cannot delete, patch or update the project's namespaces.
func (p *RTBTestSuite) TestCreateNamespacesRoleCanOnlyCreateNamespaces() {
	client := p.newSubSession()

	user := p.createUser(client, "testuser", "user")
	p1 := p.createProject(client)
	p2 := p.createProject(client)
	p.bindToProject(client, user, "create-ns", p1)
	p.bindToProject(client, user, "create-ns", p2)
	ns := p.createNamespace(client, p.projectName(p1))

	testUser, err := client.AsUser(user)
	p.Require().NoError(err)

	// Wait for every permission the role does grant, so the denials below come from Kubernetes RBAC rather than from
	// permissions that haven't propagated yet. manage-namespaces on p2 means the webhook would allow the moves.
	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:     "create",
			Resource: "namespaces",
		},
		namespaceAttributes("get", ns.Name),
		manageNamespacesAttributes(p.projectName(p1)),
		manageNamespacesAttributes(p.projectName(p2)),
	})
	p.Require().NoError(err)

	_, err = p.createNamespaceAsUser(client, testUser, p1)
	p.Require().NoError(err)

	err = p.namespaceResource(testUser).Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden, got: %v", err)

	_, err = p.moveNamespaceWithPatch(testUser, ns.Name, p2)
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden, got: %v", err)

	_, err = p.moveNamespaceWithUpdate(testUser, ns.Name, p2)
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden, got: %v", err)
}

// TestAllNamespaceVerbsCanMoveNamespace tests that a user with every verb on namespaces in two projects can move a
// namespace from one project to the other, and that access to the namespace follows it: members of only the old
// project lose access and members of only the new project gain it.
func (p *RTBTestSuite) TestAllNamespaceVerbsCanMoveNamespace() {
	client := p.newSubSession()

	user := p.createUser(client, "testuser", "user")
	sourceUser := p.createUser(client, "sourceuser", "user")
	targetUser := p.createUser(client, "targetuser", "user")
	p1 := p.createProject(client)
	p2 := p.createProject(client)
	roleTemplateID := p.createProjectRoleTemplate(client, namespaceRule("*"))
	p.bindToProject(client, user, roleTemplateID, p1)
	p.bindToProject(client, user, roleTemplateID, p2)
	p.bindToProject(client, sourceUser, "read-only", p1)
	p.bindToProject(client, targetUser, "read-only", p2)
	ns := p.createNamespace(client, p.projectName(p1))

	testUser, err := client.AsUser(user)
	p.Require().NoError(err)
	testSourceUser, err := client.AsUser(sourceUser)
	p.Require().NoError(err)
	testTargetUser, err := client.AsUser(targetUser)
	p.Require().NoError(err)

	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		namespaceAttributes("patch", ns.Name),
		manageNamespacesAttributes(p.projectName(p2)),
	})
	p.Require().NoError(err)

	// Wait for the source project member's access, so losing it after the move is a real change.
	err = extauthz.WaitForAllowed(testSourceUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		namespaceAttributes("get", ns.Name),
	})
	p.Require().NoError(err)

	_, err = p.namespaceResource(testTargetUser).Get(context.TODO(), ns.Name, metav1.GetOptions{})
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden before the move, got: %v", err)

	moved, err := p.moveNamespaceWithPatch(testUser, ns.Name, p2)
	p.Require().NoError(err)
	p.Require().Equal(p.projectAnnotation(p2), moved.GetAnnotations()[projectIDAnnotation])

	err = extauthz.WaitForAllowed(testTargetUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		namespaceAttributes("get", ns.Name),
	})
	p.Require().NoError(err)

	sourceResource := p.namespaceResource(testSourceUser)
	p.Require().EventuallyWithT(func(c *assert.CollectT) {
		_, err := sourceResource.Get(context.TODO(), ns.Name, metav1.GetOptions{})
		assert.Truef(c, apierrors.IsForbidden(err), "expected forbidden, got: %v", err)
	}, 2*time.Minute, 2*time.Second, "waiting for the source project member to lose access to the moved namespace")
}

// TestDeleteNamespacesRoleCanDeleteButNotCreate tests that a user with only delete on namespaces can delete the
// project's namespaces but cannot create new ones.
func (p *RTBTestSuite) TestDeleteNamespacesRoleCanDeleteButNotCreate() {
	client := p.newSubSession()

	user := p.createUser(client, "testuser", "user")
	project := p.createProject(client)
	roleTemplateID := p.createProjectRoleTemplate(client, namespaceRule("delete"))
	p.bindToProject(client, user, roleTemplateID, project)
	ns := p.createNamespace(client, p.projectName(project))

	testUser, err := client.AsUser(user)
	p.Require().NoError(err)

	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		namespaceAttributes("delete", ns.Name),
	})
	p.Require().NoError(err)

	_, err = p.createNamespaceAsUser(client, testUser, project)
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden, got: %v", err)

	err = p.namespaceResource(testUser).Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
	p.Require().NoError(err)
	p.Require().NoError(p.waitForNamespaceDeleted(client, ns.Name))
}

// TestUpdateNamespacesRoleCanMoveNamespace tests that a user with only update on namespaces in two projects can move
// a namespace from one project to the other.
func (p *RTBTestSuite) TestUpdateNamespacesRoleCanMoveNamespace() {
	client := p.newSubSession()

	user := p.createUser(client, "testuser", "user")
	p1 := p.createProject(client)
	p2 := p.createProject(client)
	roleTemplateID := p.createProjectRoleTemplate(client, namespaceRule("update"))
	p.bindToProject(client, user, roleTemplateID, p1)
	p.bindToProject(client, user, roleTemplateID, p2)
	ns := p.createNamespace(client, p.projectName(p1))

	testUser, err := client.AsUser(user)
	p.Require().NoError(err)

	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		namespaceAttributes("get", ns.Name),
		namespaceAttributes("update", ns.Name),
		manageNamespacesAttributes(p.projectName(p2)),
	})
	p.Require().NoError(err)

	moved, err := p.moveNamespaceWithUpdate(testUser, ns.Name, p2)
	p.Require().NoError(err)
	p.Require().Equal(p.projectAnnotation(p2), moved.GetAnnotations()[projectIDAnnotation])
}

// TestNoNamespaceVerbsGetsReadOnlyNamespaceAccess tests that a project member whose RoleTemplate has no rules for
// namespaces can still get the project's namespaces, but cannot change them, create new ones, or see namespaces in
// other projects.
func (p *RTBTestSuite) TestNoNamespaceVerbsGetsReadOnlyNamespaceAccess() {
	client := p.newSubSession()

	user := p.createUser(client, "testuser", "user")
	project := p.createProject(client)
	otherProject := p.createProject(client)
	roleTemplateID := p.createProjectRoleTemplate(client, management.PolicyRule{
		APIGroups: []string{""},
		Resources: []string{"configmaps"},
		Verbs:     []string{"get", "list", "watch"},
	})
	p.bindToProject(client, user, roleTemplateID, project)
	ns := p.createNamespace(client, p.projectName(project))
	otherNS := p.createNamespace(client, p.projectName(otherProject))

	testUser, err := client.AsUser(user)
	p.Require().NoError(err)

	err = extauthz.WaitForAllowed(testUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		namespaceAttributes("get", ns.Name),
	})
	p.Require().NoError(err)

	nsResource := p.namespaceResource(testUser)
	_, err = nsResource.Get(context.TODO(), ns.Name, metav1.GetOptions{})
	p.Require().NoError(err)

	_, err = nsResource.Patch(context.TODO(), ns.Name, k8stypes.MergePatchType, []byte(`{"metadata":{"labels":{"test":"value"}}}`), metav1.PatchOptions{})
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden, got: %v", err)

	err = nsResource.Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden, got: %v", err)

	_, err = p.createNamespaceAsUser(client, testUser, project)
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden, got: %v", err)

	_, err = nsResource.Get(context.TODO(), otherNS.Name, metav1.GetOptions{})
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden, got: %v", err)
}

// TestInheritedNamespaceVerbs tests that namespace verbs a RoleTemplate inherits are handled like its own rules:
// inheriting delete allows deleting the project's namespaces, and inheriting only create does not.
func (p *RTBTestSuite) TestInheritedNamespaceVerbs() {
	client := p.newSubSession()

	createUser := p.createUser(client, "createuser", "user")
	deleteUser := p.createUser(client, "deleteuser", "user")
	project := p.createProject(client)
	createRoleTemplateID := p.createProjectRoleTemplate(client, namespaceRule("create"))
	deleteRoleTemplateID := p.createProjectRoleTemplate(client, namespaceRule("delete"))
	p.bindToProject(client, createUser, p.createInheritingRoleTemplate(client, createRoleTemplateID), project)
	p.bindToProject(client, deleteUser, p.createInheritingRoleTemplate(client, deleteRoleTemplateID), project)
	ns := p.createNamespace(client, p.projectName(project))

	testCreateUser, err := client.AsUser(createUser)
	p.Require().NoError(err)
	testDeleteUser, err := client.AsUser(deleteUser)
	p.Require().NoError(err)

	err = extauthz.WaitForAllowed(testCreateUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		{
			Verb:     "create",
			Resource: "namespaces",
		},
		namespaceAttributes("get", ns.Name),
	})
	p.Require().NoError(err)

	err = p.namespaceResource(testCreateUser).Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
	p.Require().Error(err)
	p.Require().True(apierrors.IsForbidden(err), "expected forbidden, got: %v", err)

	err = extauthz.WaitForAllowed(testDeleteUser, p.downstreamClusterID, []*authzv1.ResourceAttributes{
		namespaceAttributes("delete", ns.Name),
	})
	p.Require().NoError(err)

	err = p.namespaceResource(testDeleteUser).Delete(context.TODO(), ns.Name, metav1.DeleteOptions{})
	p.Require().NoError(err)
	p.Require().NoError(p.waitForNamespaceDeleted(client, ns.Name))
}
