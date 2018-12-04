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

	binaryPath := path.Join(binDir(), d.DriverName)
	tmpPath := binaryPath + "-tmp"
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

	err = os.Rename(tmpPath, binaryPath)
	if err != nil {
		return "", errors.Wrapf(err, "Couldn't copy driver %v to %v", d.Name(), binaryPath)
	}

	return binaryPath, nil
}
