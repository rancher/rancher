package settings

import (
	"fmt"
	"strings"

	"github.com/rancher/rancher/pkg/utils"
)

func GetSystemNamespacesList() ([]string, error) {
	var systemNamespacesSetting Setting

	if utils.IsAgentOnly() {
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
