//go:build !linux
// +build !linux

package jailer

import (
	"fmt"
	"os/exec"
	"runtime"
)

// JailCommand does nothing in this implementation because the actual jailing of commands
// only occurs on Linux systems.
func JailCommand(cmd *exec.Cmd, _ string) (*exec.Cmd, error) {
	return nil, fmt.Errorf("not jailing command %v, unsupported on %s", cmd.Args, runtime.GOOS)
}
