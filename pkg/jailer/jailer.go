package jailer

import (
	"context"
	"fmt"
	"io"
	"net/http"
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

	if os.Getenv("MACHINE_REPO") != "rancher/machine" {
		err := processCustomMachineTarball()
		if err != nil {
			logrus.Debugf("CreateJail: error returned when processing custom machine tarball: %s\n", err)
			logrus.Debugf("CreateJail: using rancher/machine default version for jail [%s]", name)
		}
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
		logrus.Debugf("CreateJail: output from jail script for [%s]: [%v]", name, string(out))
	} else {
		logrus.Debugf("CreateJail: no output from jail script for [%s]", name)
	}
	if err != nil {
		if strings.HasSuffix(err.Error(), "signal: killed") {
			return errors.WithMessage(err, "error running the jail command: timed out waiting for the script to complete")
		}
		return errors.WithMessage(err, "error running the jail command")
	}
	return nil
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

func downloadCustomMachineTarball(filepath string, url string) (err error) {
	// Create the tarball
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloadCustomMachineTarball: bad http status returned when fetching tarball: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func processCustomMachineTarball() error {
	customMachineVersion := fmt.Sprintf("rancher-machine-%s.tar.gz", os.Getenv("ARCH"))
	customRepo := os.Getenv("MACHINE_REPO")
	tarballPath := path.Join(BaseJailPath, customMachineVersion)
	builtURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", os.Getenv("MACHINE_REPO"), os.Getenv("CATTLE_MACHINE_VERSION"), customMachineVersion)
	// RUN curl -sLf https://github.com/${MACHINE_REPO}/releases/download/${CATTLE_MACHINE_VERSION}/rancher-machine-${ARCH}.tar.gz | tar xvzf - -C /usr/bin && \
	err := downloadCustomMachineTarball(tarballPath, builtURL)
	if err != nil {
		logrus.Warnf("processCustomMachineTarball: failed to download custom machine tarball [%s] from [%s]", customMachineVersion, customRepo)
		return err
	}
	// TODO: replace with golang native tarball extraction
	tarArgs := []string{"xvzf", fmt.Sprintf(tarballPath), "-C", "/usr/bin"}
	extract := exec.Command("tar", tarArgs...)
	err = extract.Start()
	if err != nil {
		logrus.Warnf("processCustomMachineTarball: failed to start custom tarball extraction of %s, returned error: %s", tarballPath, err)
		return nil
	}
	err = extract.Wait()
	if err != nil {
		logrus.Warnf("processCustomMachineTarball: failed to extract custom tarball %s, returned error: %s", tarballPath, err)
		return err
	}
	customMachineBinaryPath := "/usr/bin/rancher-machine"
	machineBinaryDest := fmt.Sprintf("%s/%s", BaseJailPath, "driver-jail/usr/bin/rancher-machine")
	err = os.Remove(machineBinaryDest)
	if err != nil {
		logrus.Warnf("processCustomMachineTarball: failed to delete embedded rancher-machine binary from [%s]", machineBinaryDest)
		return err
	}
	err = os.Link(customMachineBinaryPath, machineBinaryDest)
	if err != nil {
		logrus.Warnf("processCustomMachineTarball: failed to link custom rancher-machine binary from [%s] to [%s]", customMachineBinaryPath, machineBinaryDest)
		return err
	}
	return nil
}
