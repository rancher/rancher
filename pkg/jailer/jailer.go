package jailer

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const BaseJailPath = "/opt/jail"

// CreateJail sets up the named directory for use with chroot
func CreateJail(name string) error {
	logrus.Debugf("Creating jail for %v", name)
	_, err := os.Stat(path.Join(BaseJailPath, name))
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

// GetUserCred looks up the user and provides it in syscall.Credential
func GetUserCred() (*syscall.Credential, error) {
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
