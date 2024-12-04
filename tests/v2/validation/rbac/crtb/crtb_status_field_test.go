//go:build (validation || infra.any || cluster.any || stress) && !sanity && !extended

package crtb

import (
	"context"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rbacv2 "github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type CRTBStatusFieldTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (crtbs *CRTBStatusFieldTestSuite) TearDownSuite() {
	crtbs.session.Cleanup()
}

func (crtbs *CRTBStatusFieldTestSuite) SetupSuite() {
	crtbs.session = session.NewSession()

	client, err := rancher.NewClient("", crtbs.session)
	require.NoError(crtbs.T(), err)
	crtbs.client = client

	log.Info("Getting cluster name from the config file and append cluster details in crtbs")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(crtbs.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(crtbs.client, clusterName)
	require.NoError(crtbs.T(), err, "Error getting cluster ID")
	crtbs.cluster, err = crtbs.client.Management.Cluster.ByID(clusterID)
	require.NoError(crtbs.T(), err)
}

func (crtbs *CRTBStatusFieldTestSuite) TestCreateCRTBAndVerifyStatusField() {
	subSession := crtbs.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a project and a namespace in the project")
	adminProject, _, err := projects.CreateProjectAndNamespace(crtbs.client, crtbs.cluster.ID)
	require.NoError(crtbs.T(), err)

	log.Infof("Create and add a standard user to downstream cluster with role Cluster Owner")
	user, _, err := rbac.AddUserWithRoleToCluster(crtbs.client, rbac.StandardUser.String(), rbac.ClusterOwner.String(), crtbs.cluster, adminProject)
	require.NoError(crtbs.T(), err)
	userID := user.Resource.ID

	log.Info("Verify that the CRTB status field and the sub-fields are correct")
	userCRTBList, err := rbac.GetClusterRoleTemplateBindings(crtbs.client, userID)
	require.NoError(crtbs.T(), err)
	userCRTB := userCRTBList[0]
	err = verifyClusterRoleTemplateBindingStatusField(&userCRTB)
	require.NoError(crtbs.T(), err)
}

func (crtbs *CRTBStatusFieldTestSuite) TestCRTBStatusFieldKubectlExplain() {
	subSession := crtbs.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Checking Status field is present in kubectl explain command output")
	explainCmd := []string{"kubectl", "explain", "clusterroletemplatebindings"}
	output, err := kubectl.Command(crtbs.client, nil, "local", explainCmd, "")
	require.NoError(crtbs.T(), err, "Error executing kubectl explain clusterroletemplatebinding")
	require.Contains(crtbs.T(), output, "status", "Status field not present in kubectl explain command output")
}

func (crtbs *CRTBStatusFieldTestSuite) TestCRTBStatusFieldKubectlDescribe() {
	subSession := crtbs.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a project and a namespace in the project")
	adminProject, _, err := projects.CreateProjectAndNamespace(crtbs.client, crtbs.cluster.ID)
	require.NoError(crtbs.T(), err)

	log.Infof("Create and add a standard user to downstream cluster with role Cluster Owner")
	user, _, err := rbac.AddUserWithRoleToCluster(crtbs.client, rbac.StandardUser.String(), rbac.ClusterOwner.String(), crtbs.cluster, adminProject)
	require.NoError(crtbs.T(), err)
	userID := user.Resource.ID

	userCRTBList, err := rbac.GetClusterRoleTemplateBindings(crtbs.client, userID)
	require.NoError(crtbs.T(), err)
	userCRTB := userCRTBList[0]

	log.Info("Checking Status Field is present in kubectl describe command output")
	describeCmd := []string{"kubectl", "describe", "clusterroletemplatebindings", "-n", crtbs.cluster.ID, userCRTB.Name}
	output, err := kubectl.Command(crtbs.client, nil, "local", describeCmd, "")
	require.NoError(crtbs.T(), err, "Error executing kubectl describe for CRTB %s", userCRTB.Name)
	require.Contains(crtbs.T(), output, "Status:", "Status field not present in kubectl describe command output")
}

func (crtbs *CRTBStatusFieldTestSuite) TestCRTBStatusFieldReconciliation() {
	subSession := crtbs.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a project and a namespace in the project")
	adminProject, _, err := projects.CreateProjectAndNamespace(crtbs.client, crtbs.cluster.ID)
	require.NoError(crtbs.T(), err)

	log.Infof("Create and add a standard user to downstream cluster with role Cluster Owner")
	user, _, err := rbac.AddUserWithRoleToCluster(crtbs.client, rbac.StandardUser.String(), rbac.ClusterOwner.String(), crtbs.cluster, adminProject)
	require.NoError(crtbs.T(), err)
	userID := user.Resource.ID

	userCRTBList, err := rbac.GetClusterRoleTemplateBindings(crtbs.client, userID)
	require.NoError(crtbs.T(), err)

	log.Info("Add environment variable CATTLE_RESYNC_DEFAULT and set it to 60 seconds")
	err = deployment.UpdateOrRemoveEnvVarForDeployment(crtbs.client, cattleSystemNamespace, deploymentName, deploymentEnvVarName, "60")
	require.NoError(crtbs.T(), err, "Failed to add environment variable")

	log.Info("Verify that CRTB resourceVersion and generation have not been updated upon reconciliation")
	initialCRTB := userCRTBList[0]
	initialResourceVersion := initialCRTB.ResourceVersion
	initialGeneration := initialCRTB.Generation

	err = wait.PollUntilContextTimeout(context.Background(), defaults.TwoMinuteTimeout, defaults.TwoMinuteTimeout, false, func(ctx context.Context) (bool, error) {
		updatedUserCRTBList, err := rbac.GetClusterRoleTemplateBindings(crtbs.client, userID)
		if err != nil {
			return false, err
		}

		updatedUserCRTB := updatedUserCRTBList[0]
		require.Equal(crtbs.T(), initialResourceVersion, updatedUserCRTB.ResourceVersion)
		require.Equal(crtbs.T(), initialGeneration, updatedUserCRTB.Generation)
		return true, nil
	})
	require.NoError(crtbs.T(), err, "Error after resync period")

	log.Info("Remove environment variable CATTLE_RESYNC_DEFAULT")
	err = deployment.UpdateOrRemoveEnvVarForDeployment(crtbs.client, cattleSystemNamespace, deploymentName, deploymentEnvVarName, "")
	require.NoError(crtbs.T(), err, "Failed to remove environment variable")
}

func (crtbs *CRTBStatusFieldTestSuite) TestUpdateCRTBAndVerifyStatusField() {
	subSession := crtbs.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a custom cluster role template")
	customClusterRole := &v3.RoleTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: customClusterRoleName,
			Labels: map[string]string{
				"app": "mock-cluster-owner",
			},
			Annotations: map[string]string{
				"management.cattle.io/creator": "norman",
			},
		},
		Context: "cluster",
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				NonResourceURLs: []string{"*"},
				Verbs:           []string{"*"},
			},
		},
	}
	createdCustomClusterRoleTemplate, err := rbacv2.CreateRoleTemplate(crtbs.client, customClusterRole)
	require.NoError(crtbs.T(), err)

	log.Info("Create a project and a namespace in the project")
	adminProject, _, err := projects.CreateProjectAndNamespace(crtbs.client, crtbs.cluster.ID)
	require.NoError(crtbs.T(), err)

	log.Infof("Create and add a standard user to downstream cluster with custom cluster role template")
	user, _, err := rbac.AddUserWithRoleToCluster(crtbs.client, rbac.StandardUser.String(), createdCustomClusterRoleTemplate.Name, crtbs.cluster, adminProject)
	require.NoError(crtbs.T(), err)
	userID := user.Resource.ID

	log.Info("Verify that the CRTB status field and the sub-fields are correct")
	userCRTBList, err := rbac.GetClusterRoleTemplateBindings(crtbs.client, userID)
	require.NoError(crtbs.T(), err)
	userCRTB := userCRTBList[0]
	err = verifyClusterRoleTemplateBindingStatusField(&userCRTB)
	require.NoError(crtbs.T(), err)

	log.Info("Adding dummy label to CRTB to trigger resync after custom cluster role template is deleted in next step")
	updatedUserCRTB := userCRTB.DeepCopy()
	updatedUserCRTB.Labels = map[string]string{
		"dummy": "dummy-label",
	}
	_, err = rbacv2.UpdateClusterRoleTemplateBindings(crtbs.client, &userCRTB, updatedUserCRTB)
	require.NoError(crtbs.T(), err)

	log.Info("Deleting custom cluster role template")
	err = rbacv2.DeleteRoletemplate(crtbs.client, customClusterRole.Name)
	require.NoError(crtbs.T(), err)

	log.Info("Verifying CRTB Status field after deleting custom cluster role template")
	err = wait.PollUntilContextTimeout(context.Background(), defaults.OneMinuteTimeout, defaults.OneMinuteTimeout, false, func(ctx context.Context) (bool, error) {
		updatedCRTBList, err := rbac.GetClusterRoleTemplateBindings(crtbs.client, userID)
		require.NoError(crtbs.T(), err)
		updatedUserCRTB := updatedCRTBList[0]
		if updatedUserCRTB.Status.Summary == "Error" {
			require.Equal(crtbs.T(), FalseConditionStatus, updatedUserCRTB.Status.LocalConditions[2].Status)
			require.Equal(crtbs.T(), FalseConditionStatus, updatedUserCRTB.Status.RemoteConditions[1].Status)
			return true, nil
		}
		return false, nil
	})
	require.NoError(crtbs.T(), err, "Error verifying CRTB Status field after deleting custom cluster role template")
}

func TestCRTBStatusFieldTestSuite(t *testing.T) {
	suite.Run(t, new(CRTBStatusFieldTestSuite))
}