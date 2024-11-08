package utils

import (
	"fmt"
	"net"
	"sort"
	"strings"

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
	if net.ParseIP(address) != nil && strings.Count(address, ":") >= 2 {
		return true
	}
	return false
}
