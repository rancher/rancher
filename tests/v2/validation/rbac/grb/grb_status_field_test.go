//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package globalrolebindings

import (
	"context"
	"fmt"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (grbs *GlobalRoleBindingStatusFieldTestSuite) TestGlobalRoleBindingStatusFieldError() {
	subSession := grbs.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Create a user with non-admin custom global role.")
	createdGlobalRole, createdUser, err := createGlobalRoleAndUser(grbs.client)
	require.NoError(grbs.T(), err)
	globalRoleName := createdGlobalRole.Name

	log.Info("Verify that the global role binding is created for the user.")
	grb, err := rbac.GetGlobalRoleBindingByUserAndRole(grbs.client, createdUser.ID, globalRoleName)
	require.NoError(grbs.T(), err)
	require.NotEmpty(grbs.T(), grb, "Global Role Binding not found for the user")

	log.Info("Verify that the global role binding status field and the sub-fields are correct.")
	err = verifyGlobalRoleBindingStatusField(grb, false)
	require.NoError(grbs.T(), err)

	previousUpdateTime, err := time.Parse(time.RFC3339, grb.Status.LastUpdateTime)
	require.NoError(grbs.T(), err)
	previousTransitionTimes := make(map[string]*metav1.Time)
	for _, condition := range grb.Status.LocalConditions {
		if condition.LastTransitionTime.IsZero() {
			log.Errorf("Condition %s has an uninitialized LastTransitionTime", condition.Type)
		} else {
			log.Infof("Condition %s has LastTransitionTime: %v", condition.Type, condition.LastTransitionTime.Time)
		}
		previousTransitionTimes[condition.Type] = condition.LastTransitionTime.DeepCopy()
	}

	log.Info("Add a dummy finalizer to the global role binding to prevent deletion.")
	grb.Finalizers = append(grb.Finalizers, dummyFinalizer)
	updateFinalizerGrb, err := grbs.client.WranglerContext.Mgmt.GlobalRoleBinding().Update(grb)
	require.NoError(grbs.T(), err)
	err = kwait.PollUntilContextTimeout(context.TODO(), defaults.TenSecondTimeout, defaults.OneMinuteTimeout, false, func(ctx context.Context) (bool, error) {
		updateFinalizerGrb, err = grbs.client.WranglerContext.Mgmt.GlobalRoleBinding().Get(grb.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, finalizer := range updateFinalizerGrb.Finalizers {
			if finalizer == dummyFinalizer {
				return true, nil
			}
		}
		return false, nil
	})
	require.NoError(grbs.T(), err)

	log.Info("Delete the global role to simulate an error condition in the global role binding status field.")
	err = grbs.client.WranglerContext.Mgmt.GlobalRole().Delete(globalRoleName, &metav1.DeleteOptions{})
	require.NoError(grbs.T(), err)
	err = kwait.PollUntilContextTimeout(context.TODO(), defaults.OneMinuteTimeout, defaults.OneMinuteTimeout, false, func(ctx context.Context) (bool, error) {
		_, err := grbs.client.WranglerContext.Mgmt.GlobalRole().Get(globalRoleName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
	require.NoError(grbs.T(), err)

	log.Info("Verify the global role binding status field summary for ", grb.Name)
	var updatedGrb *v3.GlobalRoleBinding
	err = kwait.PollUntilContextTimeout(context.TODO(), defaults.OneMinuteTimeout, defaults.OneMinuteTimeout, false, func(ctx context.Context) (bool, error) {
		updatedGrb, err = grbs.client.WranglerContext.Mgmt.GlobalRoleBinding().Get(grb.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if updatedGrb.Status.Summary == ErrorConditionStatus {
			return true, nil
		}
		return false, nil
	})
	require.NoError(grbs.T(), err)

	updatedStatus := updatedGrb.Status
	require.Equal(grbs.T(), ErrorConditionStatus, updatedStatus.Summary)
	require.Equal(grbs.T(), ErrorConditionStatus, updatedStatus.SummaryLocal)

	log.Info("Verify the global role binding status field LastUpdateTime for ", updatedGrb.Name)
	updatedTime, err := time.Parse(time.RFC3339, updatedStatus.LastUpdateTime)
	require.NoError(grbs.T(), err)
	require.NotEqual(grbs.T(), previousUpdateTime, updatedTime)

	log.Info("Verify the global role binding status field localConditions for ", updatedGrb.Name)
	notFoundMessage := fmt.Sprintf("globalroles.management.cattle.io %q not found", globalRoleName)
	expectedConditions := map[string]struct {
		Status             metav1.ConditionStatus
		Reason             string
		ExpectedTransition metav1.Time
		Message            string
	}{
		"ClusterPermissionsReconciled":    {FalseConditionStatus, failedToGetGlobalRoleReason, *previousTransitionTimes[ClusterPermissionsReconciled], notFoundMessage},
		"NamespacedRoleBindingReconciled": {FalseConditionStatus, failedToGetGlobalRoleReason, *previousTransitionTimes[NamespacedRoleBindingReconciled], notFoundMessage},
		"GlobalRoleBindingReconciled":     {TrueConditionStatus, GlobalRoleBindingReconciled, *previousTransitionTimes[GlobalRoleBindingReconciled], ""},
	}

	for _, condition := range updatedStatus.LocalConditions {
		expected, exists := expectedConditions[condition.Type]
		if exists {
			require.Equal(grbs.T(), expected.Status, condition.Status, "Local condition status mismatch for type %s", condition.Type)
			require.Equal(grbs.T(), expected.Reason, condition.Reason, "Local condition reason mismatch for type %s", condition.Type)
			require.Equal(grbs.T(), expected.Message, condition.Message, "Local condition message mismatch for type %s", condition.Type)

			if condition.Type == GlobalRoleBindingReconciled {
				// BUG: https://github.com/rancher/rancher/issues/48809
				// require.Equal(grbs.T(), expected.ExpectedTransition, condition.LastTransitionTime, "Expected LastTransitionTime for 'GlobalRoleBindingReconciled' to remain the same, but it changed.")
			} else {
				require.NotEqual(grbs.T(), expected.ExpectedTransition, condition.LastTransitionTime, "Expected LastTransitionTime for condition '%s' to change, but it remained the same.", condition.Type)
			}
		}
	}

	log.Info("Remove the finalizer and verify that the global role binding is deleted.")
	updatedGrb.Finalizers = nil
	_, err = grbs.client.WranglerContext.Mgmt.GlobalRoleBinding().Update(updatedGrb)
	require.NoError(grbs.T(), err)

	err = kwait.PollUntilContextTimeout(context.TODO(), defaults.FiveHundredMillisecondTimeout, defaults.OneMinuteTimeout, false, func(ctx context.Context) (bool, error) {
		_, err := grbs.client.WranglerContext.Mgmt.GlobalRoleBinding().Get(updatedGrb.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
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

func TestGlobalRoleBindingStatusFieldTestSuite(t *testing.T) {
	suite.Run(t, new(GlobalRoleBindingStatusFieldTestSuite))
}
