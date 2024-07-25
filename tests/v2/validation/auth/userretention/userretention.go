package userretention

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/rancher/tests/v2/actions/auth"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/sirupsen/logrus"
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
	logrus.Info("Setting up user retention settings TO : ")
	logrus.Infof("DisableAfterValue as %s, DeleteAfterValue as %s, UserRetentionCronValue as %s, DryRunValue as %s", disableAfterValue, deleteAfterValue, userRetentionCronValue, dryRunValue)
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
	logrus.Info("User retention settings setup completed")
	return nil
}

func updateUserRetentionSettings(client *rancher.Client, settingName string, settingValue string) error {
	logrus.Infof("Updating setting: %s, value: %s", settingName, settingValue)
	setting, err := client.WranglerContext.Mgmt.Setting().Get(settingName, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get setting %s: %w", settingName, err)
	}

	setting.Value = settingValue
	_, err = client.WranglerContext.Mgmt.Setting().Update(setting)
	if err != nil {
		return fmt.Errorf("failed to update setting %s: %w", settingName, err)
	}
	return nil
}

func pollUserStatus(rancherClient *rancher.Client, userID string, expectedStatus bool) error {
	logrus.Infof("Polling user status for user %s, expected status: %v", userID, expectedStatus)
	ctx, cancel := context.WithTimeout(context.Background(), defaultWaitDuration)
	defer cancel()

	adminID, err := users.GetUserIDByName(rancherClient, "admin")
	if err != nil {
		return fmt.Errorf("failed to get admin user ID: %v", err)
	}
	adminUser, err := rancherClient.Management.User.ByID(adminID)
	if err != nil {
		return fmt.Errorf("failed to get admin user: %v", err)
	}
	adminUser.Password = rancherClient.RancherConfig.AdminPassword

	return wait.PollUntilContextTimeout(ctx, pollInterval, defaultWaitDuration, true, func(ctx context.Context) (bool, error) {
		logrus.Info("Logging in with default admin user")
		_, err := auth.GetUserAfterLogin(rancherClient, *adminUser)
		if err != nil {
			return false, fmt.Errorf("failed to login with admin user: %v", err)
		}

		logrus.Info("Searching for the user status using the admin client")
		user, err := rancherClient.Management.User.ByID(userID)
		if err != nil {
			return false, fmt.Errorf("failed to get user by ID: %v", err)
		}
		if user.Enabled == nil {
			return false, fmt.Errorf("user.Enabled is nil")
		}

		currentStatus := *user.Enabled
		logrus.Infof("Current user status: %v, Expected status: %v", currentStatus, expectedStatus)
		return currentStatus == expectedStatus, nil
	})
}
