package grb

import (
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/users"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	deploymentNamespace                                       = "cattle-system"
	deploymentName                                            = "rancher"
	deploymentEnvVarName                                      = "CATTLE_RESYNC_DEFAULT"
	dummyFinalizer                                            = "dummy.example.com"
	FalseConditionStatus               metav1.ConditionStatus = "False"
	TrueConditionStatus                metav1.ConditionStatus = "True"
	ErrorConditionStatus                                      = "Error"
	failedToGetGlobalRoleReason                               = "FailedToGetGlobalRole"
	CompletedSummary                                          = "Completed"
	ClusterPermissionsReconciled                              = "ClusterPermissionsReconciled"
	GlobalRoleBindingReconciled                               = "GlobalRoleBindingReconciled"
	NamespacedRoleBindingReconciled                           = "NamespacedRoleBindingReconciled"
	FleetWorkspacePermissionReconciled                        = "FleetWorkspacePermissionReconciled"
	ClusterAdminRoleExists                                    = "ClusterAdminRoleExists"
)

var (
	customGlobalRole = v3.GlobalRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"management.cattle.io"},
				Resources: []string{"clusters"},
				Verbs:     []string{"*"},
			},
		},
	}
)

func createGlobalRoleAndUser(client *rancher.Client) (*v3.GlobalRole, *management.User, error) {
	customGlobalRole.Name = namegen.AppendRandomString("testgr")
	createdGlobalRole, err := client.WranglerContext.Mgmt.GlobalRole().Create(&customGlobalRole)
	if err != nil {
		return nil, nil, err
	}

	createdGlobalRole, err = rbac.GetGlobalRoleByName(client, createdGlobalRole.Name)
	if err != nil {
		return nil, nil, err
	}

	createdUser, err := users.CreateUserWithRole(client, users.UserConfig(), rbac.StandardUser.String(), customGlobalRole.Name)
	if err != nil {
		return nil, nil, err
	}

	return createdGlobalRole, createdUser, err
}

func verifyGlobalRoleBindingStatusField(grb *v3.GlobalRoleBinding, isAdminGlobalRole bool) error {
	status := grb.Status

	_, err := time.Parse(time.RFC3339, status.LastUpdateTime)
	if err != nil {
		return fmt.Errorf("lastUpdateTime is invalid: %w", err)
	}

	requiredLocalConditions := []string{
		ClusterPermissionsReconciled,
		GlobalRoleBindingReconciled,
		NamespacedRoleBindingReconciled,
	}
	for _, condition := range status.LocalConditions {
		for _, reqType := range requiredLocalConditions {
			if condition.Type == reqType {
				if condition.Status != TrueConditionStatus {
					return fmt.Errorf("%s condition is not True. Actual status: %s", reqType, condition.Status)
				}

				if condition.LastTransitionTime.IsZero() {
					return fmt.Errorf("%s lastTransitionTime is not set or invalid", reqType)
				}

				if condition.Message != "" {
					return fmt.Errorf("%s message should be empty. Actual message: %s", reqType, condition.Message)
				}

				if condition.Reason != condition.Type {
					return fmt.Errorf("Expected: %s, Actual: %s", condition.Type, condition.Reason)
				}
			}
		}
	}

	if status.ObservedGenerationLocal != 1 {
		return fmt.Errorf("observedGenerationLocal is not 1, found: %d", status.ObservedGenerationLocal)
	}

	if status.Summary != CompletedSummary || status.SummaryLocal != CompletedSummary {
		return fmt.Errorf("summary or summaryLocal is not 'Completed'")
	}

	if isAdminGlobalRole {
		if status.RemoteConditions != nil {
			for _, condition := range status.RemoteConditions {
				if condition.Type == ClusterAdminRoleExists && condition.Status != TrueConditionStatus {
					return fmt.Errorf("ClusterAdminRoleExists condition is not True. Actual status: %s", condition.Status)
				}

				if condition.LastTransitionTime.IsZero() {
					return fmt.Errorf("%s lastTransitionTime is not set or invalid", ClusterAdminRoleExists)
				}

				if condition.Message != "" {
					return fmt.Errorf("%s message should be empty. Actual message: %s", ClusterAdminRoleExists, condition.Message)
				}

				if condition.Reason != condition.Type {
					return fmt.Errorf("Expected: %s, Actual: %s", condition.Type, condition.Reason)
				}
			}
		}

		if status.ObservedGenerationRemote != 1 {
			return fmt.Errorf("observedGenerationRemote is not 1, found: %d", status.ObservedGenerationRemote)
		}

		if status.SummaryRemote != CompletedSummary {
			return fmt.Errorf("summaryRemote is not 'Completed'")
		}
	}

	return nil
}
