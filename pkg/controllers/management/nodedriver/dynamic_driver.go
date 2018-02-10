package nodedriver

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Driver struct {
	builtin bool
	url     string
	hash    string
	name    string
}

func NewDriver(builtin bool, name, url, hash string) *Driver {
	d := &Driver{
		builtin: builtin,
		name:    name,
		url:     url,
		hash:    hash,
	}
	if !strings.HasPrefix(d.name, "docker-machine-driver-") {
		d.name = "docker-machine-driver-" + d.name
	}
	return d
}

func (d *Driver) Name() string {
	return d.name
}

func (d *Driver) Hash() string {
	return d.hash
}

func (d *Driver) Checksum() string {
	return d.name
}

func (d *Driver) FriendlyName() string {
	return strings.TrimPrefix(d.name, "docker-machine-driver-")
}

func (d *Driver) Remove() error {
	cacheFilePrefix := d.cacheFile()
	content, err := ioutil.ReadFile(cacheFilePrefix)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return err
	}

	dest := path.Join(binDir(), string(content))
	os.Remove(dest)
	os.Remove(cacheFilePrefix + "-" + string(content))
	os.Remove(cacheFilePrefix)

	return nil
}

func (d *Driver) Stage() error {
	if err := d.getError(); err != nil {
		return err
	}

	return d.setError(d.stage())
}

func (d *Driver) setError(err error) error {
	errFile := d.cacheFile() + ".error"

	if err != nil {
		os.MkdirAll(path.Dir(errFile), 0700)
		ioutil.WriteFile(errFile, []byte(err.Error()), 0600)
	}
	return err
}

func (d *Driver) getError() error {
	errFile := d.cacheFile() + ".error"

	if content, err := ioutil.ReadFile(errFile); err == nil {
		logrus.Errorf("Returning previous error: %s", content)
		d.ClearError()
		return errors.New(string(content))
	}

	return nil
}

func (d *Driver) ClearError() {
	errFile := d.cacheFile() + ".error"
	os.Remove(errFile)
}

func (d *Driver) stage() error {
	if d.builtin {
		return nil
	}

	cacheFilePrefix := d.cacheFile()

	driverName, err := isInstalled(cacheFilePrefix)
	if err != nil || driverName != "" {
		d.name = driverName
		return err
	}

	tempFile, err := ioutil.TempFile("", "machine-driver")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	hasher, err := getHasher(d.hash)
	if err != nil {
		return err
	}

	downloadDest := io.Writer(tempFile)
	if hasher != nil {
		downloadDest = io.MultiWriter(tempFile, hasher)
	}

	if err := d.download(downloadDest); err != nil {
		return err
	}

	if got, ok := compare(hasher, d.hash); !ok {
		return fmt.Errorf("hash does not match, got %s, expected %s", got, d.hash)
	}

	if err := tempFile.Close(); err != nil {
		return err
	}

	driverName, err = d.copyBinary(cacheFilePrefix, tempFile.Name())
	if err != nil {
		return err
	}

	d.name = driverName
	return nil
}

func (d *Driver) Exists() bool {
	if d.name == "" {
		return false
	}
	if d.builtin {
		return true
	}
	binaryPath := path.Join(binDir(), d.name)
	_, err := os.Stat(binaryPath)
	return err == nil
}

func (d *Driver) Install() error {
	if d.builtin {
		return nil
	}

	binaryPath := path.Join(binDir(), d.name)
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
	return errors.Wrapf(err, "Couldn't copy driver %v to %v", d.Name(), binaryPath)
}

func isElf(input string) bool {
	f, err := os.Open(input)
	if err != nil {
		return false
	}
	defer f.Close()

	elf := make([]byte, 4)
	if _, err := f.Read(elf); err != nil {
		return false
	}

	return bytes.Compare(elf, []byte{0x7f, 0x45, 0x4c, 0x46}) == 0
}

func (d *Driver) copyBinary(cacheFile, input string) (string, error) {
	temp, err := ioutil.TempDir("", "machine-driver-extract")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)

	file := ""
	driverName := ""

	if isElf(input) {
		file = input
		u, err := url.Parse(d.url)
		if err != nil {
			return "", err
		}
		driverName = strings.Split(path.Base(u.Path), "_")[0]
		if !strings.HasPrefix(driverName, "docker-machine-driver-") {
			return "", fmt.Errorf("invalid URL %s, path should be of the format docker-machine-driver-*", d.url)
		}
	} else {
		if err := exec.Command("tar", "xvf", input, "-C", temp).Run(); err != nil {
			if err := exec.Command("unzip", "-o", input, "-d", temp).Run(); err != nil {
				return "", fmt.Errorf("failed to extract")
			}
		}
	}

	filepath.Walk(temp, filepath.WalkFunc(func(p string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if strings.HasPrefix(path.Base(p), "docker-machine-driver-") {
			file = p
		}

		return nil
	}))

	if file == "" {
		return "", fmt.Errorf("failed to find machine driver in archive. There must be a file of form docker-machine-driver*")
	}

	if driverName == "" {
		driverName = path.Base(file)
	}
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := os.MkdirAll(path.Dir(cacheFile), 0755); err != nil {
		return "", err
	}

	dest, err := os.Create(cacheFile + "-" + driverName)
	if err != nil {
		return "", err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, f); err != nil {
		return "", err
	}

	logrus.Infof("Found driver %s", driverName)
	return driverName, ioutil.WriteFile(cacheFile, []byte(driverName), 0644)
}

func (d *Driver) srcBinName() string {
	return d.cacheFile() + "-" + d.name
}

func binDir() string {
	dest := os.Getenv("GMS_BIN_DIR")
	if dest != "" {
		return dest
	}
	return "./management-state/bin"
}

func compare(hash hash.Hash, value string) (string, bool) {
	if hash == nil {
		return "", true
	}

	got := hex.EncodeToString(hash.Sum([]byte{}))
	expected := strings.TrimSpace(strings.ToLower(value))

	return got, got == expected
}

func getHasher(hash string) (hash.Hash, error) {
	switch len(hash) {
	case 0:
		return nil, nil
	case 32:
		return md5.New(), nil
	case 40:
		return sha1.New(), nil
	case 64:
		return sha256.New(), nil
	case 128:
		return sha512.New(), nil
	}

	return nil, fmt.Errorf("invalid hash format: %s", hash)
}

func (d *Driver) download(dest io.Writer) error {
	logrus.Infof("Download %s", d.url)
	resp, err := http.Get(d.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(dest, resp.Body)
	return err
}

func (d *Driver) cacheFile() string {
	key := sha256Bytes([]byte(d.url + d.hash))

	base := os.Getenv("CATTLE_HOME")
	if base == "" {
		base = "./management-state"
	}

	return path.Join(base, "machine-drivers", key)
}

func isInstalled(file string) (string, error) {
	content, err := ioutil.ReadFile(file)
	if os.IsNotExist(err) {
		return "", nil
	}
	return strings.TrimSpace(string(content)), err
}

func sha256Bytes(content []byte) string {
	hash := sha256.New()
	io.Copy(hash, bytes.NewBuffer(content))
	return hex.EncodeToString(hash.Sum([]byte{}))
}
