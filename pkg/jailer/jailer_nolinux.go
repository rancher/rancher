//go:build !linux
// +build !linux

package jailer

import (
	"os/exec"
	"runtime"

	"github.com/sirupsen/logrus"
)

func JailCommand(cmd *exec.Cmd, jailPath string) (*exec.Cmd, error) {
	logrus.Warnf("not jailing command %v, unsupported on %s", cmd.Args, runtime.GOOS)
	return cmd, nil
}
