package drivers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
)

type DynamicDriver struct {
	BaseDriver
	AssetDir string
}

var DockerMachineDriverPrefix = "docker-machine-driver-"

func NewDynamicDriver(builtin bool, name, url, hash string) (*DynamicDriver, error) {
	d := &DynamicDriver{
		BaseDriver: BaseDriver{
			Builtin:      builtin,
			DriverName:   name,
			URL:          url,
			DriverHash:   hash,
			BinaryPrefix: DockerMachineDriverPrefix,
			DevMode:      os.Getenv("CATTLE_DEV_MODE") != "",
		},
	}
	if !strings.HasPrefix(d.DriverName, DockerMachineDriverPrefix) {
		d.DriverName = DockerMachineDriverPrefix + d.DriverName
	}
	if len(hash) > 0 && d.getHasher() == nil {
		return nil, fmt.Errorf("invalid hash format: %s", hash)
	}
	return d, nil
}

func (d *DynamicDriver) Install() error {
	if d.Builtin {
		return nil
	}

	if err := d.copyTo(d.binName()); err != nil {
		return err
	}

	if d.DevMode {
		return nil
	}

	return d.copyTo(d.assetPath())
}

func (d *DynamicDriver) assetPath() string {
	if d.AssetDir != "" {
		return filepath.Join(d.AssetDir, d.DriverName)
	}
	return filepath.Join(settings.UIDashboardPath.Get(), "assets", d.DriverName)
}

// Valid returns whether the driver binary is correctly downloaded and installed
func (d *DynamicDriver) Valid() bool {
	if !d.Exists() {
		return false
	} else if d.Builtin {
		return true
	}

	expectedHash, err := os.ReadFile(d.srcBinHashFile())
	if err != nil {
		return false
	}
	hasher := sha256.New()

	f, err := os.Open(d.assetPath())
	if err != nil {
		return false
	}
	defer f.Close()

	if _, err := io.Copy(hasher, f); err != nil {
		return false
	}
	_, matches := compare(hasher, string(expectedHash))
	return matches
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

// GetDriverDownloadURL checks for a local version of the driver to download for air-gapped installs.
// If no local version is found or CATTLE_DEV_MODE is set, then the URL from the node driver is returned.
func GetDriverDownloadURL(nd *v3.NodeDriver) (string, string, error) {
	if os.Getenv("CATTLE_DEV_MODE") != "" {
		return nd.Spec.URL, nd.Spec.Checksum, nil
	}

	driverName := nd.Spec.DisplayName
	if driverName == "" {
		driverName = nd.Name
	}
	if !strings.HasPrefix(driverName, DockerMachineDriverPrefix) {
		driverName = DockerMachineDriverPrefix + driverName
	}

	path := filepath.Join(settings.UIDashboardPath.Get(), "assets", driverName)
	if _, err := os.Stat(path); err != nil {
		return nd.Spec.URL, nd.Spec.Checksum, nil
	}

	hash, err := sha256sum(path)
	if err != nil {
		return "", "", err
	}

	return fmt.Sprintf("%s/assets/%s", settings.ServerURL.Get(), driverName), hash, nil
}

func sha256sum(path string) (string, error) {
	hasher := sha256.New()
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
