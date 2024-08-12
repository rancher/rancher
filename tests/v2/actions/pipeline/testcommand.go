package pipeline

import (
	"fmt"
	"strings"

	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
)

// WrapWithAdminRunCommand is a function that returns the go test run command with
// only admin client regex option.
func WrapWithAdminRunCommand(testCase string) string {
	adminUserRegex := strings.ReplaceAll(provisioninginput.AdminClientName.String(), " ", "_")
	return fmt.Sprintf(`-run \"%s/^%s\"`, testCase, adminUserRegex)
}
