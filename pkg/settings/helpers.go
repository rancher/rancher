package settings

import (
	"fmt"
	"strings"
)

func GetSystemNamespacesList(clusterName string) ([]string, error) {
	var systemNamespacesSetting Setting

	if clusterName != "local" {
		systemNamespacesSetting = AgentSystemNamespaces
	} else {
		systemNamespacesSetting = SystemNamespaces
	}

	systemNamespacesString := systemNamespacesSetting.Get()
	if systemNamespacesString == "" {
		return []string{}, fmt.Errorf("failed to load setting %v", systemNamespacesSetting)
	}

	return strings.Split(systemNamespacesString, ","), nil
}
