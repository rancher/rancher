package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// NewConfigurationsDir is a function that creates a directory with given directory name.
// Ignores the returned error if the directory already exists.
func NewConfigurationsDir(dirName string) (err error) {
	err = os.Mkdir(dirName, 0777)
	if err != nil {
		if strings.Contains(err.Error(), "file exists") {
			logrus.Info("configs dir already exists ", err)
			return nil
		} else {
			return
		}
	}

	return
}

type ConfigFileName string

// NewConfigFileName is a constructor function that creates a configuration yaml file name
// that returns ConfigFileName.
func NewConfigFileName(dirName string, params ...string) ConfigFileName {
	fileName := strings.Join(params, "-")

	fileNameFull := fmt.Sprintf("%v/%v.yaml", dirName, fileName)

	return ConfigFileName(fileNameFull)
}

// NewFile is a method that creates a configuration yaml file with the received file name.
func (cF ConfigFileName) NewFile(data []byte) (err error) {
	err = os.WriteFile(string(cF), data, 0644)
	if err != nil {
		return
	}

	return
}

// SetEnvironmentKey is a method that sets cattle config environment variable to the received file name.
func (cF ConfigFileName) SetEnvironmentKey() (err error) {
	configEnvironmentKey := "CATTLE_TEST_CONFIG"

	configPath, err := cF.GetWDFilePath()
	if err != nil {
		return
	}

	os.Setenv(configEnvironmentKey, configPath)
	logrus.Info(configEnvironmentKey, " is set to ", configPath)

	return
}

// GetWDFilePath is a method that returns the received file name joined with wd path.
func (cF ConfigFileName) GetWDFilePath() (string, error) {
	wd, err := os.Getwd()
	logrus.Info("wd:", wd)
	if err != nil {
		return "", err
	}

	path := fmt.Sprintf("%v/%v", wd, string(cF))

	return path, nil
}
