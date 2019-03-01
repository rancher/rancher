package jailer

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// CreateJail sets up the named directory for use with chroot
func CreateJail(name string) error {
	logrus.Debugf("Creating jail for %v", name)
	_, err := os.Stat("/opt/jail/" + name)
	if err == nil {
		return nil
	}

	cmd := exec.Command("/usr/bin/jailer.sh", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("error running the jail command: %v", string(out)))
	}
	logrus.Debugf("Output from create jail command %v", string(out))
	return nil
}
