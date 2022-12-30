package jailer

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
)

func JailCommand(cmd *exec.Cmd, jailPath string) (*exec.Cmd, error) {
	if os.Getenv("CATTLE_DEV_MODE") != "" {
		return cmd, nil
	}

	cred, err := getUserCred()
	if err != nil {
		return nil, errors.WithMessage(err, "get user cred error")
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = cred
	cmd.SysProcAttr.Chroot = jailPath
	cmd.Env = getWhitelistedEnvVars(cmd.Env)
	cmd.Env = append(cmd.Env, "PWD=/")
	cmd.Dir = "/"
	return cmd, nil
}

// getUserCred looks up the user and provides it in syscall.Credential
func getUserCred() (*syscall.Credential, error) {
	u, err := user.Current()
	if err != nil {
		uID := os.Getuid()
		u, err = user.LookupId(strconv.Itoa(uID))
		if err != nil {
			return nil, err
		}
	}

	i, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return nil, err
	}
	uid := uint32(i)

	i, err = strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return nil, err
	}
	gid := uint32(i)

	return &syscall.Credential{Uid: uid, Gid: gid}, nil
}
