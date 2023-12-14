package file

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// NewDir is a function that creates a directory with given directory name.
// Ignores the returned error if the directory already exists.
func NewDir(dirName string) (err error) {
	err = os.Mkdir(dirName, 0777)
	if err != nil && strings.Contains(err.Error(), "file exists") {
		logrus.Infof("dir already exists: %v", err)
		return nil
	}

	return
}

type Name string

// NewFile is a method that creates a file with the received file name.
func (f Name) NewFile(data []byte) (wdPath string, err error) {
	err = os.WriteFile(string(f), data, 0644)
	if err != nil {
		return
	}

	wdPath, err = f.GetWDFilePath()
	if err != nil {
		return
	}

	return
}

// GetWDFilePath is a method that returns the received file name joined with wd path.
func (f Name) GetWDFilePath() (string, error) {
	wd, err := os.Getwd()
	logrus.Info("wd:", wd)
	if err != nil {
		return "", err
	}

	path := fmt.Sprintf("%v/%v", wd, string(f))

	return path, nil
}

// SetEnvironmentKey is a method that sets given environment variable to the received file name.
func (f Name) SetEnvironmentKey(envKey string) (err error) {
	configPath, err := f.GetWDFilePath()
	if err != nil {
		return
	}

	err = os.Setenv(envKey, configPath)
	logrus.Info(envKey, " is set to ", configPath)

	return
}
