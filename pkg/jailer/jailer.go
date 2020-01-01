package jailer

import (
	"context"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
)

const BaseJailPath = "/opt/jail"

var lock = sync.Mutex{}

// CreateJail sets up the named directory for use with chroot
func CreateJail(name string) error {
	logrus.Debugf("CreateJail: called for [%s]", name)
	lock.Lock()
	defer lock.Unlock()

	jailPath := path.Join(BaseJailPath, name)

	logrus.Debugf("CreateJail: jailPath is [%s]", jailPath)
	// Check for the done file, if that exists the jail is ready to be used
	_, err := os.Stat(path.Join(jailPath, "done"))
	if err == nil {
		logrus.Debugf("CreateJail: done file found at [%s], jail is ready", path.Join(jailPath, "done"))
		return nil
	}

	// If the base dir exists without the done file rebuild the directory
	_, err = os.Stat(jailPath)
	if err == nil {
		logrus.Debugf("CreateJail: basedir for jail exists but no done file found, removing jailPath [%s]", jailPath)
		if err := os.RemoveAll(jailPath); err != nil {
			return err
		}

	}

	t := settings.JailerTimeout.Get()
	timeout, err := strconv.Atoi(t)
	if err != nil {
		timeout = 60
		logrus.Warnf("error converting jailer-timeout setting to int, using default of 60 seconds - error: [%v]", err)
	}

	logrus.Debugf("CreateJail: Running script to create jail for [%s]", name)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/usr/bin/jailer.sh", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.HasSuffix(err.Error(), "signal: killed") {
			return errors.WithMessage(err, "error running the jail command: timed out waiting for the script to complete")
		}
		return errors.WithMessage(err, "error running the jail command")
	}
	if len(out) > 0 {
		logrus.Debugf("CreateJail: output from jail script for [%s]: [%v]", name, string(out))
	} else {
		logrus.Debugf("CreateJail: no output from jail script for [%s]", name)
	}
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

func WhitelistEnvvars(envvars []string) []string {
	wl := settings.WhitelistEnvironmentVars.Get()
	envWhiteList := strings.Split(wl, ",")

	if len(envWhiteList) == 0 {
		return envvars
	}

	for _, wlVar := range envWhiteList {
		wlVar = strings.TrimSpace(wlVar)
		if val := os.Getenv(wlVar); val != "" {
			envvars = append(envvars, wlVar+"="+val)
		}
	}

	return envvars
}
