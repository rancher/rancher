package planner

import (
	"fmt"
	"strings"
)

// checkForSecretFormat returns the namespace and name from the provided value which format should be "secret://namespace:name".
// A Boolean is returned to indicate whether the provided value is in the right format,
// the returned namespace and name will be empty string if either the format is wrong or an error happens
func checkForSecretFormat(value string) (bool, string, string, error) {
	if strings.HasPrefix(value, "secret://") {
		value = strings.ReplaceAll(value, "secret://", "")
		namespaceAndName := strings.Split(value, ":")
		if len(namespaceAndName) != 2 || namespaceAndName[0] == "" || namespaceAndName[1] == "" {
			return true, "", "", fmt.Errorf("provided value must be of the format secret://namespace:name")
		}
		return true, namespaceAndName[0], namespaceAndName[1], nil
	}
	return false, "", "", nil
}
