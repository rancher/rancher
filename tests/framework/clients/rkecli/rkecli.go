package rkecli

import (
	"os/exec"

	"github.com/pkg/errors"
)

const rkeCmd = "rke"

// Up uses RKE CLI up command.
// Cluster file path arg points to the cluster.yml file, args appended to the command.
func Up(clusterFilePath string, args ...string) error {
	msg, err := exec.Command(rkeCmd, "--version").CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "rke isn't executable: [%v] ", msg)
	}

	up := "up"

	// Default rke up command
	commandArgs := []string{
		up,
		"--config",
		clusterFilePath,
	}

	commandArgs = append(commandArgs, args...)

	msg, err = exec.Command(rkeCmd, commandArgs...).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "rke up: [%v] ", string(msg))
	}

	return nil
}
