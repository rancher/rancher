package jailer

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	"github.com/rancher/rancher/pkg/settings"
)

// JailCommand configures the provided command to run inside a chroot jail specified by `jailPath`.
// If the `unprivileged-jail-user` setting is enabled, the command will be configured to run with the predefined not-root UID and GID;
// otherwise, the command will inherit the UID and GID of the current process.
// This function is a no-op if the `CATTLE_DEV_MODE` environment variable is set to a non-empty value.
func JailCommand(cmd *exec.Cmd, jailPath string) (*exec.Cmd, error) {
	if os.Getenv("CATTLE_DEV_MODE") != "" {
		return cmd, nil
	}

	var cred *syscall.Credential
	var err error
	if settings.UnprivilegedJailUser.Get() == "true" {
		// Make sure the jail directory is accessible to the jail user and group as the command
		// is likely to read/write/execute files under the jail directory.
		err = SetJailOwnership(jailPath)
		if err != nil {
			return nil, err
		}

		// Get the UID and GID to use when executing the command as the jailed user.
		cred, err = getJailUserCred()
		if err != nil {
			return nil, err
		}
	} else {
		cred, err = getUserCred()
		if err != nil {
			return nil, err
		}
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: cred,
		Chroot:     jailPath,
	}
	cmd.Env = getWhitelistedEnvVars(cmd.Env)
	cmd.Env = append(cmd.Env, "PWD=/")
	cmd.Dir = "/"
	return cmd, nil
}

// getJailUserCred creates a syscall credential using the UID and GID of the jail user.
func getJailUserCred() (*syscall.Credential, error) {
	uid, err := getUserID(JailUser)
	if err != nil {
		return nil, fmt.Errorf("error finding uid for user %s: %w", JailUser, err)
	}

	gid, err := getGroupID(JailGroup)
	if err != nil {
		return nil, fmt.Errorf("error finding GID for group %s: %w", JailGroup, err)
	}

	return &syscall.Credential{
		Uid: uint32(uid),
		Gid: uint32(gid),
	}, nil
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
