package quarantine

import (
	"fmt"
	"testing"

	"github.com/rancher/shepherd/pkg/environmentflag"
)

// Quarantine package's Test method skips the given testing T by default when it's used
// to run quarantine flag, the flag in the configuration needs to be set like:
//
// flags:
//   desiredflags: quarantined
//
// or multiple flags with quarantine option:
//
// flags:
//   desiredflags: quarantined|long|otherFlag

func Test(t *testing.T, args ...any) {
	environmentFlags := environmentflag.NewEnvironmentFlags()
	environmentflag.LoadEnvironmentFlags(environmentflag.ConfigurationFileKey, environmentFlags)

	quarantined := fmt.Sprintf("Test [%v] is quarantined, skipping:", t.Name())

	reason := append([]any{quarantined}, args...)

	if !environmentFlags.GetValue(environmentflag.Quarantined) {
		t.Skip(reason)
	}
}

