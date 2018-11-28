package drivers

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

type BaseDriver struct {
	Builtin      bool
	URL          string
	DriverHash   string
	DriverName   string
	BinaryPrefix string
}

func (d *BaseDriver) Name() string {
	return d.DriverName
}

func (d *BaseDriver) Hash() string {
	return d.DriverHash
}

func (d *BaseDriver) Checksum() string {
	return d.DriverName
}

func (d *BaseDriver) FriendlyName() string {
	return strings.TrimPrefix(d.DriverName, d.BinaryPrefix)
}

func (d *BaseDriver) Remove() error {
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

func (d *BaseDriver) Stage() error {
	if err := d.getError(); err != nil {
		return err
	}

	return d.setError(d.stage())
}

func (d *BaseDriver) setError(err error) error {
	errFile := d.cacheFile() + ".error"

	if err != nil {
		os.MkdirAll(path.Dir(errFile), 0700)
		ioutil.WriteFile(errFile, []byte(err.Error()), 0600)
	}
	return err
}

func (d *BaseDriver) getError() error {
	errFile := d.cacheFile() + ".error"

	if content, err := ioutil.ReadFile(errFile); err == nil {
		logrus.Errorf("Returning previous error: %s", content)
		d.ClearError()
		return errors.New(string(content))
	}

	return nil
}

func (d *BaseDriver) ClearError() {
	errFile := d.cacheFile() + ".error"
	os.Remove(errFile)
}

func (d *BaseDriver) stage() error {
	if d.Builtin {
		return nil
	}

	cacheFilePrefix := d.cacheFile()

	driverName, err := isInstalled(cacheFilePrefix)
	if err != nil || driverName != "" {
		d.DriverName = driverName
		return err
	}

	tempFile, err := ioutil.TempFile("", "machine-driver")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	hasher, err := getHasher(d.DriverHash)
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

	if got, ok := compare(hasher, d.DriverHash); !ok {
		return fmt.Errorf("hash does not match, got %s, expected %s", got, d.DriverHash)
	}

	if err := tempFile.Close(); err != nil {
		return err
	}

	driverName, err = d.copyBinary(cacheFilePrefix, tempFile.Name())
	if err != nil {
		return err
	}

	d.DriverName = driverName
	return nil
}

func (d *BaseDriver) Exists() bool {
	if d.DriverName == "" {
		return false
	}
	if d.Builtin {
		return true
	}
	binaryPath := path.Join(binDir(), d.DriverName)
	_, err := os.Stat(binaryPath)
	return err == nil
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
	//support unix binary and mac-os binary mach-o
	return bytes.Compare(elf, []byte{0x7f, 0x45, 0x4c, 0x46}) == 0 || bytes.Compare(elf, []byte{0xcf, 0xfa, 0xed, 0xfe}) == 0
}

func (d *BaseDriver) copyBinary(cacheFile, input string) (string, error) {
	temp, err := ioutil.TempDir("", "machine-driver-extract")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)

	file := ""
	driverName := ""

	if isElf(input) {
		file = input
		u, err := url.Parse(d.URL)
		if err != nil {
			return "", err
		}

		if !strings.HasPrefix(path.Base(u.Path), d.BinaryPrefix) {
			return "", fmt.Errorf("invalid URL %s, path should be of the format %s*", d.URL, d.BinaryPrefix)
		}

		s := strings.TrimPrefix(path.Base(u.Path), d.BinaryPrefix)
		name := strings.FieldsFunc(s, func(r rune) bool {
			return r == '-' || r == '_' || r == '.'
		})[0]

		if name == "" {
			return "", fmt.Errorf("invalid URL %s, NAME is empty, path should be of the format %sNAME", d.URL, d.BinaryPrefix)
		}
		driverName = d.BinaryPrefix + name
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

		if strings.HasPrefix(path.Base(p), d.BinaryPrefix) {
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

	driverName = strings.ToLower(driverName)
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

func (d *BaseDriver) srcBinName() string {
	return d.cacheFile() + "-" + d.DriverName
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

func (d *BaseDriver) download(dest io.Writer) error {
	logrus.Infof("Download %s", d.URL)
	resp, err := http.Get(d.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(dest, resp.Body)
	return err
}

func (d *BaseDriver) cacheFile() string {
	key := sha256Bytes([]byte(d.URL + d.DriverHash))

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
	return strings.ToLower(strings.TrimSpace(string(content))), err
}

func sha256Bytes(content []byte) string {
	hash := sha256.New()
	io.Copy(hash, bytes.NewBuffer(content))
	return hex.EncodeToString(hash.Sum([]byte{}))
}
