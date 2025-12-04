package utils

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"github.com/rancher/rancher/pkg/features"
	v1 "k8s.io/api/core/v1"
)

func FormatResourceList(resources v1.ResourceList) string {
	resourceStrings := make([]string, 0, len(resources))
	for key, value := range resources {
		resourceStrings = append(resourceStrings, fmt.Sprintf("%v=%v", key, value.String()))
	}
	// sort the results for consistent log output
	sort.Strings(resourceStrings)
	return strings.Join(resourceStrings, ",")
}

// IsPlainIPV6 will return true if the given address is a plain IPV6 address and not encapsulated or similar.
func IsPlainIPV6(address string) bool {
	ipAddr, err := netip.ParseAddr(address)
	if err != nil {
		return false
	}
	return ipAddr.Is6()
}

// IsMCMServerOnly identifies when a Rancher instance is configured as an MCM server and not an MCM Agent
func IsMCMServerOnly() bool {
	return !features.MCMAgent.Enabled() && features.MCM.Enabled()
}

// IsAgentOnly identifies when a Rancher instance is acting as only an MCM Agent
func IsAgentOnly() bool {
	return features.MCMAgent.Enabled() && !features.MCM.Enabled()
}
