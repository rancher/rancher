//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package grb

import (
	"context"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

type GlobalRoleBindingStatusFieldTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (grbs *GlobalRoleBindingStatusFieldTestSuite) TearDownSuite() {
	grbs.session.Cleanup()
}

func (grbs *GlobalRoleBindingStatusFieldTestSuite) SetupSuite() {
	grbs.session = session.NewSession()

	client, err := rancher.NewClient("", grbs.session)
	require.NoError(grbs.T(), err)
	grbs.client = client

	log.Info("Getting cluster name from the config file and append cluster details in grbs")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(grbs.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(grbs.client, clusterName)
	require.NoError(grbs.T(), err, "Error getting cluster ID")
	grbs.cluster, err = grbs.client.Management.Cluster.ByID(clusterID)
	require.NoError(grbs.T(), err)
}

func (grbs *GlobalRoleBindingStatusFieldTestSuite) TestGlobalRoleBindingStatusFieldNonAdminGlobalRole() {
	subSession := grbs.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a user with non-admin custom global role.")
	createdGlobalRole, createdUser, err := createGlobalRoleAndUser(grbs.client)
	require.NoError(grbs.T(), err)

	log.Info("Verify that the global role binding is created for the user.")
	grb, err := rbac.GetGlobalRoleBindingByUserAndRole(grbs.client, createdUser.ID, createdGlobalRole.Name)
	require.NoError(grbs.T(), err)
	require.NotEmpty(grbs.T(), grb, "Global Role Binding not found for the user")

	log.Info("Verify that the global role binding status field and the sub-fields are correct.")
	err = verifyGlobalRoleBindingStatusField(grb, false)
	require.NoError(grbs.T(), err)
}

func (grbs *GlobalRoleBindingStatusFieldTestSuite) TestGlobalRoleBindingStatusFieldAdminGlobalRole() {
	subSession := grbs.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a user with admin global role.")
	createdUser, err := users.CreateUserWithRole(grbs.client, users.UserConfig(), rbac.Admin.String())
	require.NoError(grbs.T(), err)

	log.Info("Verify that the global role binding is created for the user.")
	grb, err := rbac.GetGlobalRoleBindingByUserAndRole(grbs.client, createdUser.ID, rbac.Admin.String())
	require.NoError(grbs.T(), err)
	require.NotEmpty(grbs.T(), grb, "Global Role Binding not found for the user")

	log.Info("Verify that the global role binding status field and the sub-fields are correct.")
	err = verifyGlobalRoleBindingStatusField(grb, true)
	require.NoError(grbs.T(), err)
}

func (grbs *GlobalRoleBindingStatusFieldTestSuite) TestGlobalRoleBindingStatusFieldReconciliation() {
	subSession := grbs.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a user with non-admin custom global role.")
	createdGlobalRole, createdUser, err := createGlobalRoleAndUser(grbs.client)
	require.NoError(grbs.T(), err)

	log.Info("Verify that the global role binding is created for the user.")
	grb, err := rbac.GetGlobalRoleBindingByUserAndRole(grbs.client, createdUser.ID, createdGlobalRole.Name)
	require.NoError(grbs.T(), err)
	require.NotEmpty(grbs.T(), grb, "Global Role Binding not found for the user")

	log.Info("Add environment variable CATTLE_RESYNC_DEFAULT and set it to 60 seconds")
	err = deployment.UpdateOrRemoveEnvVarForDeployment(grbs.client, deploymentNamespace, deploymentName, deploymentEnvVarName, "60")
	require.NoError(grbs.T(), err, "Failed to add environment variable")

	log.Info("Verify that global role binding resourceVersion and generation have not been updated upon reconciliation")
	initialResourceVersion := grb.ResourceVersion
	initialGeneration := grb.Generation

	var updatedGrb *v3.GlobalRoleBinding
	err = kwait.PollUntilContextTimeout(context.TODO(), defaults.TwoMinuteTimeout, defaults.TwoMinuteTimeout, false, func(ctx context.Context) (bool, error) {
		var err error
		updatedGrb, err = rbac.GetGlobalRoleBindingByUserAndRole(grbs.client, createdUser.ID, createdGlobalRole.Name)
		if err != nil {
			return false, err
		}
		return updatedGrb.ResourceVersion == initialResourceVersion && updatedGrb.Generation == initialGeneration, nil
	})
	require.NoError(grbs.T(), err, "error during polling for global role binding")
	require.NotNil(grbs.T(), updatedGrb, "updated global role binding should not be nil")
	require.Equal(grbs.T(), initialResourceVersion, updatedGrb.ResourceVersion)
	require.Equal(grbs.T(), initialGeneration, updatedGrb.Generation)

	log.Info("Remove environment variable CATTLE_RESYNC_DEFAULT")
	err = deployment.UpdateOrRemoveEnvVarForDeployment(grbs.client, deploymentNamespace, deploymentName, deploymentEnvVarName, "")
	require.NoError(grbs.T(), err, "Failed to remove environment variable")
}

func (grbs *GlobalRoleBindingStatusFieldTestSuite) TestGlobalRoleBindingStatusFieldExplain() {
	subSession := grbs.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Verify that 'kubectl explain globalrolebinding' command includes information on the 'Status' field.")
	explainCmd := []string{"kubectl", "explain", "globalrolebinding"}
	output, err := kubectl.Command(grbs.client, nil, "local", explainCmd, "")
	require.NoError(grbs.T(), err, "Error executing 'kubectl explain globalrolebinding' command.")
	require.Contains(grbs.T(), output, "status", "The 'status' field is not present in the output of 'kubectl explain globalrolebinding'.")
}

func (grbs *GlobalRoleBindingStatusFieldTestSuite) TestGlobalRoleBindingStatusFieldDescribe() {
	subSession := grbs.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a user with admin global role.")
	createdUser, err := users.CreateUserWithRole(grbs.client, users.UserConfig(), rbac.Admin.String())
	require.NoError(grbs.T(), err)

	log.Info("Verify that the global role binding is created for the user.")
	grb, err := rbac.GetGlobalRoleBindingByUserAndRole(grbs.client, createdUser.ID, rbac.Admin.String())
	require.NoError(grbs.T(), err)
	require.NotEmpty(grbs.T(), grb, "Global Role Binding not found for the user")

	log.Info("Verify that 'kubectl describe globalrolebinding' command includes information on the 'Status' field.")
	describeCmd := []string{"kubectl", "describe", "globalrolebinding", grb.Name}
	output, err := kubectl.Command(grbs.client, nil, "local", describeCmd, "")
	require.NoError(grbs.T(), err, "Error executing kubectl describe for globalrolebinding %s", grb.Name)
	require.Contains(grbs.T(), output, "Status:", "The 'Status' field is not present in the output of kubectl describe globalrolebinding.")

	subfields := []string{
		"Last Update Time:",
		"Local Conditions:",
		"Remote Conditions:",
		"Observed Generation Local:",
		"Observed Generation Remote:",
		"Summary:",
		"Summary Local:",
		"Summary Remote:",
	}

	for _, subfield := range subfields {
		require.Contains(grbs.T(), output, subfield, "The subfield '%s' is not present under 'Status' in the output of kubectl describe globalrolebinding.", subfield)
	}
}

func TestGlobalRoleBindingStatusFieldTestSuite(t *testing.T) {
	suite.Run(t, new(GlobalRoleBindingStatusFieldTestSuite))
}
