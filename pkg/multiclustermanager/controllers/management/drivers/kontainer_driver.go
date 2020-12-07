package drivers

import (
	"io"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type KontainerDriver struct {
	BaseDriver
}

var KontainerDriverPrefix = "kontainer-engine-driver-"

func NewKontainerDriver(builtin bool, name, url, hash string) *KontainerDriver {
	d := &KontainerDriver{
		BaseDriver{
			Builtin:      builtin,
			DriverName:   name,
			URL:          url,
			DriverHash:   hash,
			BinaryPrefix: KontainerDriverPrefix,
		},
	}
	if !strings.HasPrefix(d.DriverName, KontainerDriverPrefix) {
		d.DriverName = KontainerDriverPrefix + d.DriverName
	}
	return d
}

func (d *KontainerDriver) Install() (string, error) {
	if d.Builtin {
		return "", nil
	}

	installPath := path.Join(installDir(), d.DriverName)
	tmpPath := installPath + "-tmp"
	f, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", errors.Wrapf(err, "Couldn't open %v for writing", tmpPath)
	}
	defer f.Close()

	src, err := os.Open(d.srcBinName())
	if err != nil {
		return "", errors.Wrapf(err, "Couldn't open %v for copying", d.srcBinName())
	}
	defer src.Close()

	logrus.Infof("Copying %v => %v", d.srcBinName(), tmpPath)
	_, err = io.Copy(f, src)
	if err != nil {
		return "", errors.Wrapf(err, "Couldn't copy %v to %v", d.srcBinName(), tmpPath)
	}

	err = os.Rename(tmpPath, installPath)
	if err != nil {
		return "", errors.Wrapf(err, "Couldn't copy driver %v to %v", d.Name(), installPath)
	}

	return path.Join(runDir(), d.DriverName), nil
}

func (d *KontainerDriver) Exists() bool {
	if d.DriverName == "" {
		return false
	}
	if d.Builtin {
		return true
	}
	binaryPath := path.Join(installDir(), d.DriverName)
	_, err := os.Stat(binaryPath)
	return err == nil
}

func installDir() string {
	if dl := os.Getenv("CATTLE_DEV_MODE"); dl != "" {
		return "./management-state/bin"
	}

	return "/opt/jail/driver-jail/management-state/bin"
}

func runDir() string {
	if dl := os.Getenv("CATTLE_DEV_MODE"); dl != "" {
		return "./management-state/bin"
	}

	return "/management-state/bin"
}
