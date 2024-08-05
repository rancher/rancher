package userretention

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/machine/libmachine/log"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/v2/actions/auth"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/kubeapi/rbac"
	"github.com/rancher/shepherd/extensions/users"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	disableInactiveUserAfter = "disable-inactive-user-after"
	deleteInactiveUserAfter  = "delete-inactive-user-after"
	userRetentionCron        = "user-retention-cron"
	userRetentionDryRun      = "user-retention-dry-run"
	lastloginLabel           = "cattle.io/last-login"
	defaultWaitDuration      = 70 * time.Second
	pollInterval             = 10 * time.Second
	isActive                 = true
	isInActive               = false
	webhookErrorMessage      = "admission webhook \"rancher.cattle.io.settings.management.cattle.io\" denied the request: value: Invalid value:"
)

func setupUserRetentionSettings(client *rancher.Client, disableAfterValue string, deleteAfterValue string, userRetentionCronValue string, dryRunValue string) error {
	log.Info("Setting up user retention settings TO : ")
	log.Infof("DisableAfterValue as %s, DeleteAfterValue as %s, UserRetentionCronValue as %s, DryRunValue as %s", disableAfterValue, deleteAfterValue, userRetentionCronValue, dryRunValue)
	err := updateUserRetentionSettings(client, disableInactiveUserAfter, disableAfterValue)
	if err != nil {
		return fmt.Errorf("failed to update disable-inactive-user-after setting: %w", err)
	}
	err = updateUserRetentionSettings(client, deleteInactiveUserAfter, deleteAfterValue)
	if err != nil {
		return fmt.Errorf("failed to update delete-inactive-user-after setting: %w", err)
	}

	err = updateUserRetentionSettings(client, userRetentionCron, userRetentionCronValue)
	if err != nil {
		return fmt.Errorf("failed to update user-retention-cron setting: %w", err)
	}
	err = updateUserRetentionSettings(client, userRetentionDryRun, dryRunValue)
	if err != nil {
		return fmt.Errorf("failed to update user-retention-dry-run setting: %w", err)
	}
	log.Info("User retention settings setup completed")
	return nil
}

func updateUserRetentionSettings(client *rancher.Client, settingName string, settingValue string) error {
	log.Infof("Updating setting: %s, value: %s", settingName, settingValue)
	setting, err := client.WranglerContext.Mgmt.Setting().Get(settingName, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get setting %s: %w", settingName, err)
	}

	setting.Value = settingValue
	_, err = client.WranglerContext.Mgmt.Setting().Update(setting)
	if err != nil {
		return fmt.Errorf("failed to update setting %s: %w", settingName, err)
	}
	log.Infof("Setting %s updated successfully", settingName)
	return nil
}

func getRoleBindings(rancherClient *rancher.Client, clusterID string, userID string) ([]rbacv1.RoleBinding, error) {
	log.Infof("Getting role bindings for user %s in cluster %s", userID, clusterID)
	listOpt := v1.ListOptions{}
	roleBindings, err := rbac.ListRoleBindings(rancherClient, clusterID, "", listOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch RoleBindings: %w", err)
	}

	var userRoleBindings []rbacv1.RoleBinding
	for _, rb := range roleBindings.Items {
		for _, subject := range rb.Subjects {
			if subject.Name == userID {
				userRoleBindings = append(userRoleBindings, rb)
				break
			}
		}
	}
	log.Infof("Found %d role bindings for user %s", len(userRoleBindings), userID)
	return userRoleBindings, nil
}

func getBindings(rancherClient *rancher.Client, userID string) (map[string]interface{}, error) {
	log.Infof("Getting all bindings for user %s", userID)
	bindings := make(map[string]interface{})

	roleBindings, err := getRoleBindings(rancherClient, rbac.LocalCluster, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role bindings: %w", err)
	}
	bindings["RoleBindings"] = roleBindings

	log.Info("Getting cluster role bindings")
	clusterRoleBindings, err := rbac.ListClusterRoleBindings(rancherClient, rbac.LocalCluster, v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster role bindings: %w", err)
	}
	bindings["ClusterRoleBindings"] = clusterRoleBindings.Items

	log.Info("Getting global role bindings")
	globalRoleBindings, err := rancherClient.Management.GlobalRoleBinding.ListAll(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list global role bindings: %w", err)
	}
	bindings["GlobalRoleBindings"] = globalRoleBindings.Data

	log.Info("Getting cluster role template bindings")
	clusterRoleTemplateBindings, err := rancherClient.Management.ClusterRoleTemplateBinding.List(&types.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list cluster role template bindings: %w", err)
	}
	bindings["ClusterRoleTemplateBindings"] = clusterRoleTemplateBindings.Data

	log.Info("All bindings retrieved successfully")
	return bindings, nil
}

func pollUserStatus(rancherClient *rancher.Client, userID string, expectedStatus bool) error {
	log.Infof("Polling user status for user %s, expected status: %v", userID, expectedStatus)
	ctx, cancel := context.WithTimeout(context.Background(), defaultWaitDuration)
	defer cancel()

	return wait.PollUntilContextTimeout(ctx, pollInterval, defaultWaitDuration, true, func(ctx context.Context) (bool, error) {
		log.Info("Logging in with default admin user")
		adminID, err := users.GetUserIDByName(rancherClient, "admin")
		if err != nil {
			return false, fmt.Errorf("failed to get admin user ID: %v", err)
		}
		adminUser, err := rancherClient.Management.User.ByID(adminID)
		if err != nil {
			return false, fmt.Errorf("failed to get admin user: %v", err)
		}
		adminUser.Password = rancherClient.RancherConfig.AdminPassword
		if err != nil {
			return false, fmt.Errorf("failed to get admin user password: %v", err)
		}

		_, err = auth.GetUserAfterLogin(rancherClient, *adminUser)
		if err != nil {
			return false, fmt.Errorf("failed to login with admin user: %v", err)
		}

		log.Info("Searching for the user status using the admin client")
		user, err := rancherClient.Management.User.ByID(userID)
		if err != nil {
			return false, fmt.Errorf("failed to get user by ID: %v", err)
		}
		if user.Enabled == nil {
			return false, fmt.Errorf("user.Enabled is nil")
		}

		currentStatus := *user.Enabled
		log.Infof("Current user status: %v, Expected status: %v", currentStatus, expectedStatus)
		return currentStatus == expectedStatus, nil
	})
}
