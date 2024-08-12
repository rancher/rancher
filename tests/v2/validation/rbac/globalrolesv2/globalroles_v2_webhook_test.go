//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package globalrolesv2

import (
	"regexp"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rbacapi "github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/users"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/rancher/rancher/tests/v2/actions/rbac"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GlobalRolesV2WebhookTestSuite struct {
	suite.Suite
	client       *rancher.Client
	session      *session.Session
	clusterCount int
}

func (grw *GlobalRolesV2WebhookTestSuite) TearDownSuite() {
	grw.session.Cleanup()
}

func (grw *GlobalRolesV2WebhookTestSuite) SetupSuite() {
	grw.session = session.NewSession()

	client, err := rancher.NewClient("", grw.session)
	require.NoError(grw.T(), err)

	grw.client = client
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(grw.T(), clusterName, "Cluster name to install should be set")

	clusterNames, err := clusters.ListDownstreamClusters(grw.client)
	grw.clusterCount = len(clusterNames)
	require.NoError(grw.T(), err)

}

func getCRTBFromGRBOwner(t *testing.T, client *rancher.Client, user *management.User, expectedCrtbCount int) (*v3.ClusterRoleTemplateBindingList, error, string) {
	log.Info("Verify that the global role binding is created for the user.")
	grbOwner, err := getGlobalRoleBindingForUserWrangler(client, user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, grbOwner, "Global Role Binding not found for the user")

	log.Info("Verify that the cluster role template bindings are created for the downstream clusters.")
	crtbList, err := listClusterRoleTemplateBindingsForInheritedClusterRoles(client, grbOwner, expectedCrtbCount)

	return crtbList, err, grbOwner

}

func (grw *GlobalRolesV2WebhookTestSuite) TestRemoveOwnerLabelRejectedByWebhook() {
	subSession := grw.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a global role with inheritedClusterRoles and verify removing the grb owner label is rejected.")
	user, err := createGlobalRoleAndUser(grw.client, []string{rbac.ClusterOwner.String()})
	require.NoError(grw.T(), err)

	crtbList, err, _ := getCRTBFromGRBOwner(grw.T(), grw.client, user, grw.clusterCount)
	require.NoError(grw.T(), err)

	var existingCRTB = &v3.ClusterRoleTemplateBinding{}
	for _, crtb := range crtbList.Items {
		existingCRTB = crtb.DeepCopy()
		newLabels := crtb.Labels
		if newLabels == nil {
			newLabels = make(map[string]string)
		}
		delete(newLabels, ownerLabel)
		crtbWithUpdatedLabels := &v3.ClusterRoleTemplateBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        crtb.Name,
				Namespace:   crtb.ObjectMeta.Namespace,
				Labels:      newLabels,
				Annotations: crtb.Annotations,
			},
			ClusterName:       crtb.ClusterName,
			UserName:          crtb.UserName,
			RoleTemplateName:  crtb.RoleTemplateName,
			UserPrincipalName: crtb.UserPrincipalName,
		}

		_, err = rbacapi.UpdateClusterRoleTemplateBindings(grw.client, existingCRTB, crtbWithUpdatedLabels)
		require.Error(grw.T(), err)

		expectedErrMessage := "admission webhook \"rancher.cattle.io.clusterroletemplatebindings.management.cattle.io\" denied the request: clusterroletemplatebinding.labels: Forbidden: label authz.management.cattle.io/grb-owner is immutable after creation"
		assert.Contains(grw.T(), err.Error(), expectedErrMessage)

	}

}

func (grw *GlobalRolesV2WebhookTestSuite) TestLockedRoleTemplateInInheritedClusterRole() {

	log.Info("Verify adding a locked custom cluster role template is rejected by webhook.")

	roleTemplate := &v3.RoleTemplate{}
	roleTemplate.Name = namegen.AppendRandomString("customrole")
	roleTemplate.RoleTemplateNames = []string{rbac.ClusterOwner.String()}
	roleTemplate.Context = clusterContext
	roleTemplate.Locked = true
	lockedRoleTemplate, err := rbacapi.CreateRoleTemplate(grw.client, roleTemplate)
	require.NoError(grw.T(), err)

	_, err = createGlobalRoleWithInheritedClusterRolesWrangler(grw.client, []string{lockedRoleTemplate.Name})
	require.Error(grw.T(), err)

	pattern := "^admission webhook.*" + regexp.QuoteMeta(lockedRoleTemplate.Name+"\": unable to use locked roleTemplate") + "$"
	require.Regexp(grw.T(), regexp.MustCompile(pattern), err.Error())
}

func (grw *GlobalRolesV2WebhookTestSuite) TestAddGlobalRoleWithCustomTemplateAndLockRoleTemplate() {

	log.Info("Verify adding a custom cluster role template and then locking the template is not rejected by webhook.")

	roleTemplate := &v3.RoleTemplate{}
	roleTemplate.Name = namegen.AppendRandomString("customrole")
	roleTemplate.RoleTemplateNames = []string{rbac.ClusterOwner.String()}
	roleTemplate.Context = clusterContext
	customRoleTemplate, err := rbacapi.CreateRoleTemplate(grw.client, roleTemplate)
	require.NoError(grw.T(), err)

	_, err = createGlobalRoleWithInheritedClusterRolesWrangler(grw.client, []string{customRoleTemplate.Name})
	require.NoError(grw.T(), err)

	customRoleTemplate.Locked = true
	lockCustomRoleTemplate, err := rbacapi.UpdateRoleTemplate(grw.client, customRoleTemplate)
	require.NoError(grw.T(), err)
	assert.Equal(grw.T(), lockCustomRoleTemplate.Locked, true)

}

func (grw *GlobalRolesV2WebhookTestSuite) TestDeleteCustomRoleTemplateInInheritedClusterRole() {
	log.Info("Verify deleting a custom cluster role template thats inherited by global role is rejected.")
	inheritedRoleTemplateName := namegen.AppendRandomString("customrole")
	inheritedRoleTemplate, err := grw.client.Management.RoleTemplate.Create(&management.RoleTemplate{
		Context:         "cluster",
		Name:            inheritedRoleTemplateName,
		RoleTemplateIDs: []string{rbac.ClusterOwner.String()},
	})
	require.NoError(grw.T(), err)

	_, err = createGlobalRoleWithInheritedClusterRolesWrangler(grw.client, []string{inheritedRoleTemplate.ID})
	require.NoError(grw.T(), err)

	err = rbacapi.DeleteRoletemplate(grw.client, inheritedRoleTemplate.ID)
	require.Error(grw.T(), err)

	pattern := "^admission webhook .*" + regexp.QuoteMeta("cannot be deleted because it is inherited by globalRole(s) \"") + regexp.QuoteMeta(globalRole.Name) + "\"$"
	require.Regexp(grw.T(), regexp.MustCompile(pattern), err.Error())
}

func (grw *GlobalRolesV2WebhookTestSuite) TestAddProjectRoleTemplateInInheritedClusterRole() {
	log.Info("Verify adding a project role template is rejected by webhook.")
	inheritedRoleTemplateName := namegen.AppendRandomString("customrole")
	inheritedRoleTemplate, err := grw.client.Management.RoleTemplate.Create(&management.RoleTemplate{
		Context:         projectContext,
		Name:            inheritedRoleTemplateName,
		RoleTemplateIDs: []string{rbac.ProjectOwner.String()},
	})
	require.NoError(grw.T(), err)

	_, err = createGlobalRoleWithInheritedClusterRolesWrangler(grw.client, []string{inheritedRoleTemplate.ID})
	require.Error(grw.T(), err)

	pattern := "admission webhook.*" + regexp.QuoteMeta("unable to bind a roleTemplate with non-cluster context: project")
	require.Regexp(grw.T(), regexp.MustCompile(pattern), err.Error())

}

func (grw *GlobalRolesV2WebhookTestSuite) TestRoleTemplateWithBadUserSubject() {

	log.Info("Verify creating a cluster role template binding with the label grb Owner for a new user is rejected by webhook")
	user, err := createGlobalRoleAndUser(grw.client, []string{rbac.ClusterOwner.String()})
	require.NoError(grw.T(), err)

	crtbList, err, grbOwner := getCRTBFromGRBOwner(grw.T(), grw.client, user, grw.clusterCount)
	require.NoError(grw.T(), err)

	for _, crtb := range crtbList.Items {
		log.Info("Create a new user with global role standard user and custom global role.")

		createdUser, err := users.CreateUserWithRole(grw.client, users.UserConfig(), rbac.StandardUser.String(), globalRole.Name)
		require.NoError(grw.T(), err)

		clusterRoleTemplateBinding := &v3.ClusterRoleTemplateBinding{}

		clusterRoleTemplateBinding.Name = namegen.AppendRandomString("test-")
		clusterRoleTemplateBinding.Namespace = crtb.Namespace
		clusterRoleTemplateBinding.UserName = createdUser.Name
		clusterRoleTemplateBinding.RoleTemplateName = crtb.RoleTemplateName
		clusterRoleTemplateBinding.UserPrincipalName = localPrefix + createdUser.Name
		clusterRoleTemplateBinding.Labels = crtb.Labels
		clusterRoleTemplateBinding.ClusterName = crtb.ClusterName

		createdCRTB, err := rbacapi.CreateClusterRoleTemplateBinding(grw.client, clusterRoleTemplateBinding)
		require.NoError(grw.T(), err)

		req, err := labels.NewRequirement(ownerLabel, selection.In, []string{grbOwner})
		require.NoError(grw.T(), err)

		selector := labels.NewSelector().Add(*req)

		err = crtbStatus(grw.client, createdCRTB.Name, selector)
		require.NoError(grw.T(), err, "Newly created CRTB exists and not deleted")
	}
}

func (grw *GlobalRolesV2WebhookTestSuite) TestDuplicateCRTBsAreDeleted() {

	log.Info("Verify creating a duplicate crtb and adding the grbOwner as label gets deleted.")
	user, err := createGlobalRoleAndUser(grw.client, []string{rbac.ClusterOwner.String()})
	require.NoError(grw.T(), err)

	crtbList, err, grbOwner := getCRTBFromGRBOwner(grw.T(), grw.client, user, grw.clusterCount)
	require.NoError(grw.T(), err)

	for _, crtb := range crtbList.Items {
		clusterRoleTemplateBinding := &v3.ClusterRoleTemplateBinding{}

		clusterRoleTemplateBinding.Name = namegen.AppendRandomString("test-")
		clusterRoleTemplateBinding.Namespace = crtb.Namespace
		clusterRoleTemplateBinding.UserName = user.Name
		clusterRoleTemplateBinding.RoleTemplateName = crtb.RoleTemplateName
		clusterRoleTemplateBinding.UserPrincipalName = localPrefix + user.Name
		clusterRoleTemplateBinding.Labels = crtb.Labels
		clusterRoleTemplateBinding.ClusterName = crtb.ClusterName
		createdCRTB, err := rbacapi.CreateClusterRoleTemplateBinding(grw.client, clusterRoleTemplateBinding)
		require.NoError(grw.T(), err)

		req, err := labels.NewRequirement(ownerLabel, selection.In, []string{grbOwner})
		require.NoError(grw.T(), err)

		selector := labels.NewSelector().Add(*req)

		err = crtbStatus(grw.client, createdCRTB.Name, selector)
		require.NoError(grw.T(), err, "Newly created CRTB exists and not deleted")
	}

}

func (grw *GlobalRolesV2WebhookTestSuite) TestCRTBWithLocalClusterReferenceIsDeleted() {

	log.Info("Create a global role with inheritedClusterRoles and a user added to the global role.")
	user, err := createGlobalRoleAndUser(grw.client, []string{rbac.ClusterOwner.String()})
	require.NoError(grw.T(), err)

	crtbList, err, grbOwner := getCRTBFromGRBOwner(grw.T(), grw.client, user, grw.clusterCount)
	require.NoError(grw.T(), err)

	for _, crtb := range crtbList.Items {

		log.Info("Create a new user with global role standard user and custom global role.")

		createdUser, err := users.CreateUserWithRole(grw.client, users.UserConfig(), rbac.StandardUser.String(), globalRole.Name)
		require.NoError(grw.T(), err)

		clusterRoleTemplateBinding := &v3.ClusterRoleTemplateBinding{}

		clusterRoleTemplateBinding.Name = namegen.AppendRandomString("test-")
		clusterRoleTemplateBinding.Namespace = localcluster
		clusterRoleTemplateBinding.UserName = createdUser.Name
		clusterRoleTemplateBinding.RoleTemplateName = crtb.RoleTemplateName
		clusterRoleTemplateBinding.UserPrincipalName = localPrefix + createdUser.Name
		clusterRoleTemplateBinding.Annotations = crtb.Annotations
		clusterRoleTemplateBinding.Labels = crtb.Labels
		clusterRoleTemplateBinding.ClusterName = localcluster

		createdCRTB, err := rbacapi.CreateClusterRoleTemplateBinding(grw.client, clusterRoleTemplateBinding)
		require.NoError(grw.T(), err)

		req, err := labels.NewRequirement(ownerLabel, selection.In, []string{grbOwner})
		require.NoError(grw.T(), err)
		selector := labels.NewSelector().Add(*req)

		err = crtbStatus(grw.client, createdCRTB.Name, selector)
		require.NoError(grw.T(), err, "Newly created CRTB exists and not deleted")
	}
}

func TestGlobalRolesV2WebhookTestSuite(t *testing.T) {
	suite.Run(t, new(GlobalRolesV2WebhookTestSuite))
}
