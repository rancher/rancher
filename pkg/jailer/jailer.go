package jailer

import (
	"context"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
)

const BaseJailPath = "/opt/jail"

var lock = sync.Mutex{}

// CreateJail sets up the named directory for use with chroot
func CreateJail(name string) error {
	if os.Getenv("CATTLE_DEV_MODE") != "" {
		return os.MkdirAll(path.Join(BaseJailPath, name), 0700)
	}

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
	if len(out) > 0 {
		logrus.Tracef("CreateJail: output from jail script for [%s]: [%v]", name, string(out))
	} else {
		logrus.Tracef("CreateJail: no output from jail script for [%s]", name)
	}
	if err != nil {
		if strings.HasSuffix(err.Error(), "signal: killed") {
			return errors.WithMessage(err, "error running the jail command: timed out waiting for the script to complete")
		}
		return errors.WithMessage(err, "error running the jail command")
	}
	return nil
}

func getWhitelistedEnvVars(envvars []string) []string {
	settings.IterateWhitelistedEnvVars(func(name, value string) {
		envvars = append(envvars, name+"="+value)
	})
	return envvars
}
