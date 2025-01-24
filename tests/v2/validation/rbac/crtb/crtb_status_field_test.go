//go:build (validation || infra.any || cluster.any || stress) && !sanity && !extended

package crtb

import (
	"context"
	"fmt"
	"testing"
	"time"
	"strings"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/users"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/v2/actions/rancherleader"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CRTBStatusFieldTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

const (
	cattleSystemNamespace = "cattle-system"
	defaultWaitDuration   = 60 * time.Second
)

func (crtb *CRTBStatusFieldTestSuite) TearDownSuite() {
	crtb.session.Cleanup()
}

func (crtbs *CRTBStatusFieldTestSuite) SetupSuite() {
	testSession := session.NewSession()
	crtbs.session = testSession

	client, err := rancher.NewClient("", testSession)
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
	log.Info("Create a standard user")
	user, err := users.CreateUserWithRole(crtbs.client, users.UserConfig(), "user")
	require.NoError(crtbs.T(), err)
	userID := user.Resource.ID

	log.Info("Add user to downstream cluster with role Cluster Owner")
	err = users.AddClusterRoleToUser(crtbs.client, crtbs.cluster, user, "cluster-owner", nil)
	require.NoError(crtbs.T(), err)

	userCRTBList, err := GetClusterRoleTemplateBindings(crtbs.client, userID)
	require.NoError(crtbs.T(), err)

	for _, crtb := range userCRTBList {
		log.Info("Verifying CRTB Status field localConditions for ", crtb.Name)
		expectedLocalConditions := map[string]struct {
			Status v1.ConditionStatus
			Reason string
		}{
			"SubjectExists":    {v1.ConditionStatus("True"), "SubjectExists"},
			"LabelsReconciled": {v1.ConditionStatus("True"), "LabelsReconciled"},
			"BindingExists":    {v1.ConditionStatus("True"), "BindingExists"},
		}

		for _, condition := range crtb.Status.LocalConditions {
			expected, exists := expectedLocalConditions[condition.Type]
			if exists {
				assert.Equal(crtbs.T(), expected.Status, condition.Status, "Local condition status mismatch for type %s", condition.Type)
				assert.Equal(crtbs.T(), expected.Reason, condition.Reason, "Local condition status mismatch for type %s", condition.Type)
			}
		}

		log.Info("Verifying CRTB Status field remoteConditions for ", crtb.Name)
		expectedRemoteConditions := map[string]struct {
			Status v1.ConditionStatus
			Reason string
		}{
			"CRTBLabelsUpdated":                {v1.ConditionStatus("True"), "CRTBLabelsUpdated"},
			"ClusterRolesExists":               {v1.ConditionStatus("True"), "ClusterRolesExists"},
			"ClusterRoleBindingsExists":        {v1.ConditionStatus("True"), "ClusterRoleBindingsExists"},
			"ServiceAccountImpersonatorExists": {v1.ConditionStatus("True"), "ServiceAccountImpersonatorExists"},
		}
		for _, condition := range crtb.Status.RemoteConditions {
			expected, exists := expectedRemoteConditions[condition.Type]
			if exists {
				assert.Equal(crtbs.T(), expected.Status, condition.Status, "Remote condition status mismatch for type %s", condition.Type)
				assert.Equal(crtbs.T(), expected.Reason, condition.Reason, "Remote condition reason mismatch for type %s", condition.Type)
			}
		}

		log.Info("Veryfing CRTB Status field observedGeneration for ", crtb.Name)
		expectedGeneration := int64(2)
		assert.Equal(crtbs.T(), crtb.Generation, expectedGeneration, "Local observed generation mismatch")
		assert.Equal(crtbs.T(), crtb.Generation, expectedGeneration, "Remote observed generation mismatch")

		log.Info("Verifying CRTB Status field Summary for ", crtb.Name)
		assert.Equal(crtbs.T(), "Completed", crtb.Status.Summary, "Expected status summary to be 'Completed'")
		assert.Equal(crtbs.T(), "Completed", crtb.Status.SummaryLocal, "Expected local status summary to be 'Completed'")
		assert.Equal(crtbs.T(), "Completed", crtb.Status.SummaryRemote, "Expected remote status summary to be 'Completed'")

	}
}

func (crtbs *CRTBStatusFieldTestSuite) TestCRTBStatusFieldKubectl() {
	log.Info("Create a standard user")
	user, err := users.CreateUserWithRole(crtbs.client, users.UserConfig(), "user")
	require.NoError(crtbs.T(), err)
	userID := user.Resource.ID

	log.Info("Add user to downstream cluster with role Cluster Owner")
	err = users.AddClusterRoleToUser(crtbs.client, crtbs.cluster, user, "cluster-owner", nil)
	require.NoError(crtbs.T(), err)

	userCRTBList, err := GetClusterRoleTemplateBindings(crtbs.client, userID)
	require.NoError(crtbs.T(), err)

	crtbs.T().Run(fmt.Sprintf("Checking Status field via kubectl"), func(t *testing.T) {
		lsCmd := []string{"kubectl", "explain", "clusterroletemplatebindings"}
		output, err := kubectl.Command(crtbs.client, nil, "local", lsCmd, "")
		if err != nil {
			crtbs.T().Error(err)
			return
		}
		assert.Contains(crtbs.T(), output, "status")
		require.NoError(crtbs.T(), err, "Status field not present in output")
	})

	for _, crtb := range userCRTBList {
		log.Info("Verifying CRTB Status field lastUpdateTime for ", crtb.Name)
		crtbLastUpdateTime := crtb.Status.LastUpdateTime
		_, err = time.Parse(time.RFC3339, crtbLastUpdateTime)
		assert.NoError(crtbs.T(), err, "Invalid lastUpdateTime format for CRTB: %s", crtbLastUpdateTime)

		crtbs.T().Run(fmt.Sprintf("Checking Status field via kubectl describe"), func(t *testing.T) {
			lsCmd := []string{"kubectl", "describe", "clusterroletemplatebindings", "-n", crtbs.cluster.ID, crtb.Name}
			output, err := kubectl.Command(crtbs.client, nil, "local", lsCmd, "")
			if err != nil {
				crtbs.T().Errorf("Error executing command in pod %s", err)
				return
			}
			assert.Contains(crtbs.T(), output, "Status:")
			require.NoError(crtbs.T(), err, "Status field not present in output")
		})
	}
}

func (crtbs *CRTBStatusFieldTestSuite) TestCRTBStatusFieldReconciliation() {
	log.Info("Create a standard user")
	user, err := users.CreateUserWithRole(crtbs.client, users.UserConfig(), "user")
	require.NoError(crtbs.T(), err)
	userID := user.Resource.ID

	log.Info("Add user to downstream cluster with role Cluster Owner")
	err = users.AddClusterRoleToUser(crtbs.client, crtbs.cluster, user, "cluster-owner", nil)
	require.NoError(crtbs.T(), err)

	userCRTBList, err := GetClusterRoleTemplateBindings(crtbs.client, userID)
	require.NoError(crtbs.T(), err)

	podNames, err := pods.GetPodNamesFromDeployment(crtbs.client, localCluster, cattleSystemNamespace, "rancher")
	if err != nil {
		return
	}

	var selectedPod *corev1.Pod
	for _, podName := range podNames {
		pod, err := pods.GetPodByName(crtbs.client, localCluster, cattleSystemNamespace, podName)
		if err != nil {
			log.Warnf("Pod %s not found: %v", podName, err)
			continue
		}

		if !strings.Contains(pod.Name, "webhook") {
			selectedPod = pod
			break
		}
	}

	crtbs.T().Run(fmt.Sprintf("Checking that CATTLE_RESYNC_DEFAULT is set to 60 seconds"), func(t *testing.T) {
		lsCmd := []string{"kubectl", "exec", "-n", "cattle-system", selectedPod.Name, "--", "printenv", "CATTLE_RESYNC_DEFAULT"}
		output, err := kubectl.Command(crtbs.client, nil, "local", lsCmd, "")
		if err != nil {
			log.Info("Error retrieving CATTLE_RESYNC_DEFAULT env var: ", err)
			crtbs.T().Error(err)
			return
		}
		assert.Contains(crtbs.T(), output, "60", "CATTLE_RESYNC_DEFAULT value is not set to '60'")
		require.NoError(crtbs.T(), err, "CATTLE_RESYNC_DEFAULT value is invalid.")
	})

	log.Info("Verify that CRTB resourceVersion and generation have not been updated upon reconciliation")
	initialCRTB := userCRTBList[0]
	initialResourceVersion := initialCRTB.ResourceVersion
	initialGeneration := initialCRTB.Generation

	ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
	defer cancel()

	err = wait.PollUntilContextTimeout(ctx, defaultWaitDuration, defaultWaitDuration, false, func(ctx context.Context) (done bool, err error) {
		updatedUserCRTBList, err := GetClusterRoleTemplateBindings(crtbs.client, userID)
		if err != nil {
			return false, err
		}

		updatedUserCRTB := updatedUserCRTBList[0]
		assert.Equal(crtbs.T(), initialResourceVersion, updatedUserCRTB.ResourceVersion)
		assert.Equal(crtbs.T(), initialGeneration, updatedUserCRTB.Generation)
		return true, nil
	})
	require.NoError(crtbs.T(), err, "Error after resync period")

}

func (crtbs *CRTBStatusFieldTestSuite) TestUpdateCRTBAndVerifyStatusField() {
	log.Info("Create a standard user")
	user, err := users.CreateUserWithRole(crtbs.client, users.UserConfig(), "user")
	require.NoError(crtbs.T(), err)
	userID := user.Resource.ID

	log.Info("Create a custom cluster role template")
	customRoleName := "custom-cluster-owner"
	customRole := &v3.RoleTemplate{
		ObjectMeta: v1.ObjectMeta{
			Name: customRoleName,
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
	createdCustomRoleTemplate, err := rbac.CreateRoleTemplate(crtbs.client, customRole)
	require.NoError(crtbs.T(), err)

	log.Info("Add user to downstream cluster with created custom-cluster-owner role")
	err = users.AddClusterRoleToUser(crtbs.client, crtbs.cluster, user, createdCustomRoleTemplate.Name, nil)
	require.NoError(crtbs.T(), err)

	userCRTBList, err := GetClusterRoleTemplateBindings(crtbs.client, userID)
	require.NoError(crtbs.T(), err)
	userCRTBName := userCRTBList[0].Name

	for _, crtb := range userCRTBList {
		log.Info("Verifying CRTB Status field localConditions for ", crtb.Name)
		expectedLocalConditions := map[string]struct {
			Status v1.ConditionStatus
			Reason string
		}{
			"SubjectExists":    {v1.ConditionStatus("True"), "SubjectExists"},
			"LabelsReconciled": {v1.ConditionStatus("True"), "LabelsReconciled"},
			"BindingExists":    {v1.ConditionStatus("True"), "BindingExists"},
		}

		for _, condition := range crtb.Status.LocalConditions {
			expected, exists := expectedLocalConditions[condition.Type]
			if exists {
				assert.Equal(crtbs.T(), expected.Status, condition.Status, "Local condition status mismatch for type %s", condition.Type)
				assert.Equal(crtbs.T(), expected.Reason, condition.Reason, "Local condition status mismatch for type %s", condition.Type)
			}
		}

		log.Info("Verifying CRTB Status field remoteConditions for ", crtb.Name)
		expectedRemoteConditions := map[string]struct {
			Status v1.ConditionStatus
			Reason string
		}{
			"CRTBLabelsUpdated":                {v1.ConditionStatus("True"), "CRTBLabelsUpdated"},
			"ClusterRolesExists":               {v1.ConditionStatus("True"), "ClusterRolesExists"},
			"ClusterRoleBindingsExists":        {v1.ConditionStatus("True"), "ClusterRoleBindingsExists"},
			"ServiceAccountImpersonatorExists": {v1.ConditionStatus("True"), "ServiceAccountImpersonatorExists"},
		}
		for _, condition := range crtb.Status.RemoteConditions {
			expected, exists := expectedRemoteConditions[condition.Type]
			if exists {
				assert.Equal(crtbs.T(), expected.Status, condition.Status, "Remote condition status mismatch for type %s", condition.Type)
				assert.Equal(crtbs.T(), expected.Reason, condition.Reason, "Remote condition reason mismatch for type %s", condition.Type)
			}
		}

		log.Info("Veryfing CRTB Status field observedGeneration for ", crtb.Name)
		expectedGeneration := int64(2)
		assert.Equal(crtbs.T(), crtb.Generation, expectedGeneration, "Local observed generation mismatch")
		assert.Equal(crtbs.T(), crtb.Generation, expectedGeneration, "Remote observed generation mismatch")

		log.Info("Verifying CRTB Status field Summary for ", crtb.Name)
		assert.Equal(crtbs.T(), "Completed", crtb.Status.Summary, "Expected status summary to be 'Completed'")
		assert.Equal(crtbs.T(), "Completed", crtb.Status.SummaryLocal, "Expected local status summary to be 'Completed'")
		assert.Equal(crtbs.T(), "Completed", crtb.Status.SummaryRemote, "Expected remote status summary to be 'Completed'")

		
		log.Info("Adding dummy label to CRTB to trigger resync after role template is deleted in next step")
		newLabel := map[string]string{
			"dummy": "dummy-label",
		}
		crtbWithUpdatedLabels := &v3.ClusterRoleTemplateBinding{
			ObjectMeta: v1.ObjectMeta{
				Name:      crtb.Name,
				Namespace: crtb.ObjectMeta.Namespace,
				Labels:    newLabel,
			},
			ClusterName:       crtb.ClusterName,
			UserName:          crtb.UserName,
			RoleTemplateName:  crtb.RoleTemplateName,
			UserPrincipalName: crtb.UserPrincipalName,
		}
		_, err := rbac.UpdateClusterRoleTemplateBindings(crtbs.client, &crtb, crtbWithUpdatedLabels)
		require.NoError(crtbs.T(), err)

	}

	log.Info("Deleting custom cluster role template")
	err = rbac.DeleteRoletemplate(crtbs.client, customRole.Name)
	require.NoError(crtbs.T(), err)

	log.Info("Verifying CRTB Status field after deleting custom cluster role template")
	err = wait.PollUntilContextTimeout(context.Background(), 60*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
		updatedCRTBList, err := GetClusterRoleTemplateBindings(crtbs.client, userID)
		require.NoError(crtbs.T(), err)
		updatedUserCRTB := updatedCRTBList[0]
		if updatedUserCRTB.Status.Summary == "Error" {
			assert.Equal(crtbs.T(), v1.ConditionStatus("False"), updatedUserCRTB.Status.LocalConditions[2].Status)
			assert.Equal(crtbs.T(), v1.ConditionStatus("False"), updatedUserCRTB.Status.RemoteConditions[1].Status)
			return true, nil
		}

		return false, nil
	})
	require.NoError(crtbs.T(), err, "Error verifying CRTB Status field after deleting custom cluster role template")

	log.Info("Verifying error in rancher pod logs")
	logCaptureStartTime := time.Now()
	leaderPodName, err := rancherleader.GetRancherLeaderPodName(crtbs.client)
	require.NoError(crtbs.T(), err)
	errorRegex := `\[ERROR\] error syncing '` + crtbs.cluster.ID + `/` + userCRTBName + `': handler auth-prov-v2-crtb: roletemplates.management.cattle.io "custom-cluster-owner" not found, requeuing`
	err = pods.CheckPodLogsForErrors(crtbs.client, localCluster, leaderPodName, cattleSystemNamespace, errorRegex, logCaptureStartTime)
	require.Error(crtbs.T(), err)
}

func TestCRTBStatusFieldTestSuite(t *testing.T) {
	suite.Run(t, new(CRTBStatusFieldTestSuite))
}
