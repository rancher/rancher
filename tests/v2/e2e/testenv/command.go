package testenv

import (
	"os/exec"

	"github.com/pkg/errors"
)

// runBinary executes a binary cmd and returns the stdOutput
func RunBinary(binaryName string, args ...string) ([]byte, error) {
	cmd := exec.Command(binaryName, args...)
	stdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return stdOutput, errors.Wrapf(err, "%s cmd, failed with the following error: %s", cmd.Args, string(stdOutput))
	}
	return stdOutput, nil
}
