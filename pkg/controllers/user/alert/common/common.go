package common

import (
	"fmt"
)

func GetRuleID(groupID string, ruleName string) string {
	return fmt.Sprintf("%s_%s", groupID, ruleName)
}

func GetGroupID(namespace, name string) string {
	return fmt.Sprintf("%s:%s", namespace, name)
}

func GetAlertManagerSecretName(appName string) string {
	return fmt.Sprintf("alertmanager-%s", appName)
}

func GetAlertManagerDaemonsetName(appName string) string {
	return fmt.Sprintf("alertmanager-%s", appName)
}
