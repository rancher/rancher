package drivers

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/jailer"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
)

type DynamicDriver struct {
	BaseDriver
}

var DockerMachineDriverPrefix = "docker-machine-driver-"

const ExecutionTimeout = time.Second * 5

func NewDynamicDriver(builtin bool, name, url, hash string) *DynamicDriver {
	d := &DynamicDriver{
		BaseDriver{
			Builtin:      builtin,
			DriverName:   name,
			URL:          url,
			DriverHash:   hash,
			BinaryPrefix: DockerMachineDriverPrefix,
		},
	}
	if !strings.HasPrefix(d.DriverName, DockerMachineDriverPrefix) {
		d.DriverName = DockerMachineDriverPrefix + d.DriverName
	}
	return d
}

func (d *DynamicDriver) Install() error {
	if d.Builtin {
		return nil
	}

	if err := d.copyTo(d.binName()); err != nil {
		return err
	}

	if os.Getenv("CATTLE_DEV_MODE") != "" {
		return nil
	}

	return d.copyTo(fmt.Sprintf("%s/assets/%s", settings.UIPath.Get(), d.DriverName))
}

func (d *BaseDriver) copyTo(dest string) error {
	binaryPath := d.binName()
	tmpPath := binaryPath + "-tmp"
	f, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return errors.Wrapf(err, "Couldn't open %v for writing", tmpPath)
	}
	defer f.Close()

	src, err := os.Open(d.srcBinName())
	if err != nil {
		return errors.Wrapf(err, "Couldn't open %v for copying", d.srcBinName())
	}
	defer src.Close()

	logrus.Infof("Copying %v => %v", d.srcBinName(), tmpPath)
	_, err = io.Copy(f, src)
	if err != nil {
		return errors.Wrapf(err, "Couldn't copy %v to %v", d.srcBinName(), tmpPath)
	}

	err = os.Rename(tmpPath, dest)
	if err != nil {
		return errors.Wrapf(err, "Couldn't copy driver %v to %v", d.Name(), dest)
	}

	return nil
}

// Executable is will return nil if the driver can be executed
func (d *BaseDriver) Executable() error {
	if d.DriverName == "" {
		return fmt.Errorf("empty driver name")
	}

	if d.Builtin {
		return nil
	}
	logrus.Debugf("Checking if driver %s is executable", d.DriverName)
	binaryPath := d.binName()
	info, err := os.Lstat(binaryPath)
	if err != nil {
		return fmt.Errorf("driver %s not found", binaryPath)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("driver %s is not a regular file", binaryPath)
	}

	// prepare a jail
	jailName := fmt.Sprintf("exec-jail-%s", d.DriverName)
	jailPath := path.Join(jailer.BaseJailPath, jailName)
	err = jailer.CreateJail(jailName)
	if err != nil {
		return fmt.Errorf("couldn't create exec-jail: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), ExecutionTimeout)
	defer cancel()

	binaryPathInJail := path.Join("/var/lib/rancher/management-state/bin", d.DriverName)
	cmd, err := jailer.JailCommand(exec.CommandContext(ctx, binaryPathInJail), jailPath)
	if err != nil {
		return fmt.Errorf("error jailing command: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return errors.Wrapf(err, "driver binary %s couldn't execute", binaryPathInJail)
	}

	// We don't care about the exit code, just want to make sure we can execute the binary
	_ = cmd.Wait()

	// Remove the jail to ensure it is rebuilt each time,
	// as the hard link still points to the old driver after downloading a new version
	info, err = os.Lstat(jailPath)
	if err == nil && info.IsDir() {
		if err := os.RemoveAll(jailPath); err != nil {
			return fmt.Errorf("couldn't remove exec-jail: %w", err)
		}
	}

	return nil
}
