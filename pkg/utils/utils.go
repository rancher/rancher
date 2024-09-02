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

// FormatPrefix converts the provided string into a form suitable for use as a
// generateName prefix.
//
// It does this by converting to lower-case and appending a "-" character.
func FormatPrefix(s string) string {
	if s == "" {
		return s
	}

	s = strings.ToLower(s)
	if !strings.HasSuffix(s, "-") {
		s = s + "-"
	}

	return s
}

// IsPlainIPV6 will return true if the given address is a plain IPV6 address and not encapsulated or similar.
func IsPlainIPV6(address string) bool {
	if net.ParseIP(address) != nil && strings.Count(address, ":") >= 2 {
		return true
	}
	return false
}
