package drivers

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type DynamicDriver struct {
	BaseDriver
}

var DockerMachineDriverPrefix = "docker-machine-driver-"

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

	err = os.Rename(tmpPath, binaryPath)
	if err != nil {
		return errors.Wrapf(err, "Couldn't copy driver %v to %v", d.Name(), binaryPath)
	}
	return nil
}

// Executable is will return nil if the driver can be executed
func (d *BaseDriver) Executable() error {
	if d.DriverName == "" {
		return fmt.Errorf("Empty driver name")
	}

	if d.Builtin {
		return nil
	}

	binaryPath := d.binName()
	_, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("Driver %s not found", binaryPath)
	}
	err = exec.Command(binaryPath).Start()
	if err != nil {
		return errors.Wrapf(err, "Driver binary %s couldn't execute", binaryPath)
	}
	return nil
}
