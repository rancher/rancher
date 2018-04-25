package networkpolicy

import (
	"github.com/rancher/rancher/pkg/settings"
	"strings"
)

func GetSystemNamespaces() map[string]bool {
	systemNamespaces := make(map[string]bool)
	systemNamespacesStr := settings.SystemNamespaces.Get()
	if systemNamespacesStr != "" {
		splits := strings.Split(systemNamespacesStr, ",")
		for _, s := range splits {
			systemNamespaces[strings.TrimSpace(s)] = true
		}
	}
	return systemNamespaces
}
