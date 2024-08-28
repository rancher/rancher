package globalrolesv2

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rbacapi "github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/secrets"
	"github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/users"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	rbacv1 "k8s.io/api/rbac/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespacedRulesTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (ns *NamespacedRulesTestSuite) TearDownSuite() {
	ns.session.Cleanup()
}

func (ns *NamespacedRulesTestSuite) SetupSuite() {
	ns.session = session.NewSession()

	client, err := rancher.NewClient("", ns.session)
	require.NoError(ns.T(), err)

	ns.client = client
}

func (ns *NamespacedRulesTestSuite) validateNSRulesRBACResources(user *management.User, namespacedRules map[string][]rbacv1.PolicyRule) {
	log.Info("Verify that the global role binding is created for the user.")
	grbOwner, err := getGlobalRoleBindingForUserWrangler(ns.client, user.ID)
	require.NoError(ns.T(), err)
	require.NotEmpty(ns.T(), grbOwner, "Global Role Binding not found for the user")

	expectedRbCount := len(namespacedRules)
	log.Info("Verify that the role bindings are created in the local cluster for specified policy rule.")
	rbCount := 0

	for namespace := range namespacedRules {
		nameSelector := fmt.Sprintf("metadata.name=%s-%s", grbOwner, namespace)
		rbs, err := rbacapi.ListRoleBindings(ns.client, localcluster, namespace, metav1.ListOptions{FieldSelector: nameSelector})
		if namespace == "*" {
			require.Empty(ns.T(), rbs.Items, "Unexpected number of RoleBindings: Expected %d, Actual %d", 0, len(rbs.Items))
			return
		}
		require.NoError(ns.T(), err)
		rbCount += 1
	}

	require.Equal(ns.T(), expectedRbCount, rbCount, "Unexpected number of RoleBindings: Expected %d, Actual %d", expectedRbCount, rbCount)
}

func (ns *NamespacedRulesTestSuite) TestCreateUserWithNamespacedRules() {

	subSession := ns.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Validate creating a global role with namespacedRules and assign it to a user.")

	project, err := ns.client.Management.Project.Create(projects.NewProjectConfig(localcluster))
	require.NoError(ns.T(), err)

	namespaceName := namegen.AppendRandomString("testns-")
	namespace, err := namespaces.CreateNamespace(ns.client, namespaceName, "{}", map[string]string{}, map[string]string{}, project)
	require.NoError(ns.T(), err)
	log.Info("Create a global role with namespacedRules.")
	namespacedRules := map[string][]rbacv1.PolicyRule{
		namespace.Name: {
			readSecretsPolicy,
		},
	}
	createdGlobalRole, err := createGlobalRoleWithNamespacedRules(ns.client, namespacedRules)
	require.NoError(ns.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(ns.client, users.UserConfig(), rbac.StandardUser.String(), createdGlobalRole.Name)
	require.NoError(ns.T(), err)

	ns.validateNSRulesRBACResources(createdUser, namespacedRules)

	log.Info("Verify user can list secrets in the namespace from the namespaced rules")
	userClient, err := ns.client.AsUser(createdUser)
	require.NoError(ns.T(), err)

	listSecretsAsUser, err := secrets.ListSecrets(userClient, localcluster, namespace.Name, metav1.ListOptions{})
	require.NoError(ns.T(), err)
	listSecretsAsAdmin, err := secrets.ListSecrets(ns.client, localcluster, namespace.Name, metav1.ListOptions{})
	require.NoError(ns.T(), err)
	require.Equal(ns.T(), listSecretsAsAdmin.Items, listSecretsAsUser.Items)
	require.Equal(ns.T(), len(listSecretsAsAdmin.Items), len(listSecretsAsUser.Items))

	log.Info("Verify user cannot create secrets in the namespace from the namespaced rules")
	_, err = secrets.CreateSecretForCluster(userClient, &secret, localcluster, namespace.Name)
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))

	log.Info("Verify user cannot create secrets in other namespaces as well")
	_, err = secrets.ListSecrets(userClient, localcluster, globalDataNamespace, metav1.ListOptions{})
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))
}

func (ns *NamespacedRulesTestSuite) TestCreateUserWithStarAsKeyInNamespacedRules() {

	subSession := ns.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with * as key in namespacedRules.")
	namespacedRules := map[string][]rbacv1.PolicyRule{
		"*": {
			readSecretsPolicy,
		},
	}
	createdGlobalRole, err := createGlobalRoleWithNamespacedRules(ns.client, namespacedRules)
	require.NoError(ns.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(ns.client, users.UserConfig(), rbac.StandardUser.String(), createdGlobalRole.Name)
	require.NoError(ns.T(), err)

	ns.validateNSRulesRBACResources(createdUser, namespacedRules)

	log.Info("Verify user cannot list secrets in any namespace from the namespaced rules")
	userClient, err := ns.client.AsUser(createdUser)
	require.NoError(ns.T(), err)

	_, err = secrets.ListSecrets(userClient, localcluster, globalDataNamespace, metav1.ListOptions{})
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))

	log.Info("Verify user cannot create secrets in other namespaces as well")
	_, err = secrets.CreateSecretForCluster(userClient, &secret, localcluster, "default")
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))
}

func (ns *NamespacedRulesTestSuite) TestCreateUserWithStarForResourcesAndGroups() {

	subSession := ns.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with * as resources and api groups in namespacedRules in a custom namespace.")

	customNS, err := createProjectAndAddANamespace(ns.client, "ns-readall")
	require.NoError(ns.T(), err)

	namespacedRules := map[string][]rbacv1.PolicyRule{
		customNS: {
			readAllResourcesPolicy,
		},
	}
	createdGlobalRole, err := createGlobalRoleWithNamespacedRules(ns.client, namespacedRules)
	require.NoError(ns.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(ns.client, users.UserConfig(), rbac.StandardUser.String(), createdGlobalRole.Name)
	require.NoError(ns.T(), err)

	ns.validateNSRulesRBACResources(createdUser, namespacedRules)

	log.Info("Verify user can list secrets in the defined namespaced rules")
	userClient, err := ns.client.AsUser(createdUser)
	require.NoError(ns.T(), err)

	log.Info("Create secrets as an admin in the customNS and verify user can list the secret.")
	createAdminSecret, err := secrets.CreateSecretForCluster(ns.client, &secret, localcluster, customNS)
	require.NoError(ns.T(), err)
	listSecretsCustomNS, err := secrets.ListSecrets(userClient, localcluster, customNS, metav1.ListOptions{})
	require.NoError(ns.T(), err)
	require.Equal(ns.T(), 1, len(listSecretsCustomNS.Items))
	require.Equal(ns.T(), createAdminSecret.Name, listSecretsCustomNS.Items[0].Name)

	log.Info("Verify user cannot list secrets in any other namespace than the defined from the namespaced rules")
	_, err = secrets.ListSecrets(userClient, localcluster, globalDataNamespace, metav1.ListOptions{})
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))

	log.Info("Verify user cannot create secrets in other namespaces as well")
	_, err = secrets.CreateSecretForCluster(userClient, &secret, localcluster, "default")
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))
}

func (ns *NamespacedRulesTestSuite) TestCreateUserWithMultipleNSInNamespacedRules() {

	subSession := ns.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with multiple namespaces in namespacedRules.")

	customNS, err := createProjectAndAddANamespace(ns.client, "ns-readcrtbs")
	require.NoError(ns.T(), err)

	namespacedRules := map[string][]rbacv1.PolicyRule{
		globalDataNamespace: {
			readSecretsPolicy,
		},
		customNS: {
			readCRTBsPolicy},
	}

	createdGlobalRole, err := createGlobalRoleWithNamespacedRules(ns.client, namespacedRules)
	require.NoError(ns.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(ns.client, users.UserConfig(), rbac.StandardUser.String(), createdGlobalRole.Name)
	require.NoError(ns.T(), err)

	ns.validateNSRulesRBACResources(createdUser, namespacedRules)

	log.Info("Verify user can list secrets in the provided namespace from the namespaced rules for secrets. ")
	userClient, err := ns.client.AsUser(createdUser)
	require.NoError(ns.T(), err)

	listSecretsAsUser, err := secrets.ListSecrets(userClient, localcluster, globalDataNamespace, metav1.ListOptions{})
	require.NoError(ns.T(), err)
	listSecretsAsAdmin, err := secrets.ListSecrets(ns.client, localcluster, globalDataNamespace, metav1.ListOptions{})
	require.NoError(ns.T(), err)
	require.Equal(ns.T(), listSecretsAsAdmin.Items, listSecretsAsUser.Items)
	require.Equal(ns.T(), len(listSecretsAsAdmin.Items), len(listSecretsAsUser.Items))

	log.Info("Verify user cannot list secrets in the custom namespace from the namespaced rules for secrets. ")
	_, err = secrets.ListSecrets(userClient, localcluster, customNS, metav1.ListOptions{})
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))

	log.Info("Verify user can list CRTBS in the custom namespace from the namespaced rules for CRTBs. ")
	crtbListAsUser, err := userClient.WranglerContext.Mgmt.ClusterRoleTemplateBinding().List(customNS, metav1.ListOptions{})
	require.NoError(ns.T(), err)
	crtbListAsAdmin, err := ns.client.WranglerContext.Mgmt.ClusterRoleTemplateBinding().List(customNS, metav1.ListOptions{})
	require.NoError(ns.T(), err)
	require.Equal(ns.T(), len(crtbListAsAdmin.Items), len(crtbListAsUser.Items))

	log.Info("Verify user cannot list CRTBS in the globalDataNamespace from the namespaced rules for CRTBs. ")
	_, err = userClient.WranglerContext.Mgmt.ClusterRoleTemplateBinding().List(globalDataNamespace, metav1.ListOptions{})
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))
}

func (ns *NamespacedRulesTestSuite) TestUpdateGlobalRoleWithNamespacedRules() {

	subSession := ns.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Validate updating global role, creates new role bindings.")

	createdGlobalRole, err := createGlobalRoleWithNamespacedRules(ns.client, nil)
	require.NoError(ns.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(ns.client, users.UserConfig(), rbac.StandardUser.String(), createdGlobalRole.Name)
	require.NoError(ns.T(), err)

	namespacedRules := map[string][]rbacv1.PolicyRule{
		globalDataNamespace: {
			readSecretsPolicy,
		},
	}
	updatedGlobalRole := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: createdGlobalRole.Name,
		},
		NamespacedRules: namespacedRules,
	}

	log.Info("Updating global role with namespaced rules.")
	_, err = rbacapi.UpdateGlobalRole(ns.client, &updatedGlobalRole)
	require.NoError(ns.T(), err)
	ns.validateNSRulesRBACResources(createdUser, namespacedRules)

	log.Info("Verify user can list secrets in the namespace from the namespaced rules")
	userClient, err := ns.client.AsUser(createdUser)
	require.NoError(ns.T(), err)

	listSecretsAsUser, err := secrets.ListSecrets(userClient, localcluster, globalDataNamespace, metav1.ListOptions{})
	require.NoError(ns.T(), err)
	listSecretsAsAdmin, err := secrets.ListSecrets(ns.client, localcluster, globalDataNamespace, metav1.ListOptions{})
	require.NoError(ns.T(), err)
	require.Equal(ns.T(), listSecretsAsAdmin.Items, listSecretsAsUser.Items)
	require.Equal(ns.T(), len(listSecretsAsAdmin.Items), len(listSecretsAsUser.Items))

	log.Info("Updating global role by adding multiple namespaced rules.")
	customNS, err := createProjectAndAddANamespace(ns.client, "ns-readcrtbs")
	require.NoError(ns.T(), err)

	namespacedRules[customNS] = []rbacv1.PolicyRule{readCRTBsPolicy}
	updatedGlobalRoleWithMultiNamespacedRules := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: createdGlobalRole.Name,
		},
		NamespacedRules: namespacedRules,
	}

	_, err = rbacapi.UpdateGlobalRole(ns.client, &updatedGlobalRoleWithMultiNamespacedRules)
	require.NoError(ns.T(), err)

	ns.validateNSRulesRBACResources(createdUser, namespacedRules)

	log.Info("Verify user cannot list secrets in the custom namespace from the namespaced rules for secrets. ")
	_, err = secrets.ListSecrets(userClient, localcluster, customNS, metav1.ListOptions{})
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))

	log.Info("Verify user can list CRTBS in the custom namespace from the namespaced rules for CRTBs. ")
	_, err = userClient.WranglerContext.Mgmt.ClusterRoleTemplateBinding().List(customNS, metav1.ListOptions{})
	require.NoError(ns.T(), err)

	log.Info("Verify user cannot list CRTBS in the globalDataNamespace from the namespaced rules for CRTBs. ")
	_, err = userClient.WranglerContext.Mgmt.ClusterRoleTemplateBinding().List(globalDataNamespace, metav1.ListOptions{})
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))
}

func (ns *NamespacedRulesTestSuite) TestDeleteNamespacedRulesFromGlobalRole() {

	subSession := ns.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Validate delting namespaced rules from global roles, deletes the role bindings for a user.")

	namespacedRules := map[string][]rbacv1.PolicyRule{
		globalDataNamespace: {
			readSecretsPolicy,
		},
	}
	createdGlobalRole, err := createGlobalRoleWithNamespacedRules(ns.client, namespacedRules)
	require.NoError(ns.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(ns.client, users.UserConfig(), rbac.StandardUser.String(), createdGlobalRole.Name)
	require.NoError(ns.T(), err)

	ns.validateNSRulesRBACResources(createdUser, namespacedRules)

	grbOwner, err := getGlobalRoleBindingForUserWrangler(ns.client, createdUser.ID)
	require.NoError(ns.T(), err)
	nameSelector := fmt.Sprintf("metadata.name=%s-%s", grbOwner, namespace)

	deleteNamespacedRules := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: createdGlobalRole.Name,
		},
		NamespacedRules: map[string][]rbacv1.PolicyRule{},
	}

	log.Info("Delete the namespace in the namespaced rules.")
	_, err = rbacapi.UpdateGlobalRole(ns.client, &deleteNamespacedRules)
	require.NoError(ns.T(), err)

	rbs, err := rbacapi.ListRoleBindings(ns.client, localcluster, namespace, metav1.ListOptions{FieldSelector: nameSelector})
	require.NoError(ns.T(), err)
	require.Empty(ns.T(), rbs.Items)
}

func (ns *NamespacedRulesTestSuite) TestDeleteANamespaceFromNamespacedRules() {

	subSession := ns.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Validate deleting a namespace from namespaced rules.")

	readPodsNS, err := createProjectAndAddANamespace(ns.client, "custom-sa-ns")
	require.NoError(ns.T(), err)
	readCrtbsNS, err := createProjectAndAddANamespace(ns.client, "custom-crtbs-ns")
	require.NoError(ns.T(), err)
	namespacedRules := map[string][]rbacv1.PolicyRule{
		readPodsNS: {
			readPods,
		},
		readCrtbsNS: {
			readCRTBsPolicy,
		},
	}
	createdGlobalRole, err := createGlobalRoleWithNamespacedRules(ns.client, namespacedRules)
	require.NoError(ns.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(ns.client, users.UserConfig(), rbac.StandardUser.String(), createdGlobalRole.Name)
	require.NoError(ns.T(), err)

	ns.validateNSRulesRBACResources(createdUser, namespacedRules)

	log.Info("Update the global role and remove a namespace.")
	namespacedRules = map[string][]rbacv1.PolicyRule{
		readPodsNS: {
			readPods,
		},
	}

	deleteANamespace := v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: createdGlobalRole.Name,
		},
		NamespacedRules: namespacedRules,
	}

	_, err = rbacapi.UpdateGlobalRole(ns.client, &deleteANamespace)
	require.NoError(ns.T(), err)

	ns.validateNSRulesRBACResources(createdUser, namespacedRules)

	log.Info("Verify user can list pods in the namespace from the namespaced rules")
	userClient, err := ns.client.AsUser(createdUser)
	require.NoError(ns.T(), err)
	listPodsAsUser, err := userClient.WranglerContext.Core.Pod().List(readPodsNS, metav1.ListOptions{})
	require.NoError(ns.T(), err)
	listPodsAsAdmin, err := ns.client.WranglerContext.Core.Pod().List(readPodsNS, metav1.ListOptions{})
	require.NoError(ns.T(), err)
	require.Equal(ns.T(), len(listPodsAsAdmin.Items), len(listPodsAsUser.Items))
	require.Equal(ns.T(), listPodsAsAdmin.Items, listPodsAsUser.Items)

	log.Info("Verify user cannot list crtbs in the deleted namespace from the namespaced rules")
	_, err = userClient.WranglerContext.Mgmt.ClusterRoleTemplateBinding().List(readCrtbsNS, metav1.ListOptions{})
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))
}

func (ns *NamespacedRulesTestSuite) TestDeleteUserDeletesRolebindingsForNamespacedRules() {

	subSession := ns.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Validate deleting user deletes the rolebindings created for namespaced rules.")

	readPodsNS, err := createProjectAndAddANamespace(ns.client, "custom-sa-ns")
	require.NoError(ns.T(), err)

	namespacedRules := map[string][]rbacv1.PolicyRule{
		readPodsNS: {
			readPods,
		},
	}
	createdGlobalRole, err := createGlobalRoleWithNamespacedRules(ns.client, namespacedRules)
	require.NoError(ns.T(), err)

	log.Info("Create a user with global role standard user and custom global role.")
	createdUser, err := users.CreateUserWithRole(ns.client, users.UserConfig(), rbac.StandardUser.String(), createdGlobalRole.Name)
	require.NoError(ns.T(), err)

	ns.validateNSRulesRBACResources(createdUser, namespacedRules)
	grbOwner, err := getGlobalRoleBindingForUserWrangler(ns.client, createdUser.ID)
	require.NoError(ns.T(), err)
	nameSelector := fmt.Sprintf("metadata.name=%s-%s", grbOwner, namespace)

	log.Info("Deleting the user attached to the namespaced rules.")
	err = ns.client.WranglerContext.Mgmt.User().Delete(createdUser.ID, &metav1.DeleteOptions{})
	require.NoError(ns.T(), err)

	rbs, err := rbacapi.ListRoleBindings(ns.client, localcluster, namespace, metav1.ListOptions{FieldSelector: nameSelector})
	require.NoError(ns.T(), err)
	require.Empty(ns.T(), rbs.Items)
}

func (ns *NamespacedRulesTestSuite) TestWebhookRejectsEmptyVerbsInNamespacedRules() {

	subSession := ns.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with namespacedRules having empty verbs.")

	readSecretsPolicy.Verbs = []string{}
	namespacedRules := map[string][]rbacv1.PolicyRule{
		globalDataNamespace: {
			readSecretsPolicy,
		},
	}
	_, err := createGlobalRoleWithNamespacedRules(ns.client, namespacedRules)
	require.Error(ns.T(), err)
	errMessage := "admission webhook \"rancher.cattle.io.globalroles.management.cattle.io\" denied the request: globalrole.namespacedRules." + globalDataNamespace + "[0].verbs: Required value: verbs must contain at least one value"

	require.Equal(ns.T(), errMessage, err.Error())
}

func (ns *NamespacedRulesTestSuite) TestWebhookIncorrectVerbsNotRejected() {

	subSession := ns.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with namespacedRules having incorrect verbs.")

	readSecretsPolicy.Verbs = []string{"incorrect"}
	namespacedRules := map[string][]rbacv1.PolicyRule{
		globalDataNamespace: {
			readSecretsPolicy,
		},
	}
	globalRole, err := createGlobalRoleWithNamespacedRules(ns.client, namespacedRules)
	require.NoError(ns.T(), err)

	log.Info("Create a user and assign the global role.")
	createdUser, err := users.CreateUserWithRole(ns.client, users.UserConfig(), rbac.StandardUser.String(), globalRole.Name)
	require.NoError(ns.T(), err)

	ns.validateNSRulesRBACResources(createdUser, namespacedRules)

	log.Info("Verify user cannot list secrets in the defined namespaced rules")
	userClient, err := ns.client.AsUser(createdUser)
	require.NoError(ns.T(), err)

	_, err = secrets.ListSecrets(userClient, localcluster, globalDataNamespace, metav1.ListOptions{})
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))
}

func (ns *NamespacedRulesTestSuite) TestWebhookIncorrectResourcesNotRejected() {

	subSession := ns.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with namespacedRules having empty verbs.")

	readSecretsPolicy.Resources = []string{"incorrect"}
	namespacedRules := map[string][]rbacv1.PolicyRule{
		globalDataNamespace: {
			readSecretsPolicy,
		},
	}
	globalRole, err := createGlobalRoleWithNamespacedRules(ns.client, namespacedRules)
	require.NoError(ns.T(), err)

	log.Info("Create a user and assign the global role.")
	createdUser, err := users.CreateUserWithRole(ns.client, users.UserConfig(), rbac.StandardUser.String(), globalRole.Name)
	require.NoError(ns.T(), err)

	ns.validateNSRulesRBACResources(createdUser, namespacedRules)

	log.Info("Verify user cannot list secrets in the defined namespaced rules")
	userClient, err := ns.client.AsUser(createdUser)
	require.NoError(ns.T(), err)

	_, err = secrets.ListSecrets(userClient, localcluster, globalDataNamespace, metav1.ListOptions{})
	require.Error(ns.T(), err)
	require.True(ns.T(), k8sError.IsForbidden(err))
}

func TestNamespacedRulesTestSuite(t *testing.T) {
	suite.Run(t, new(NamespacedRulesTestSuite))
}
