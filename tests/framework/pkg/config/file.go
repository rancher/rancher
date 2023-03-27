package config

import (
	"fmt"
	"strings"

	"github.com/rancher/rancher/tests/framework/pkg/file"
)

// NewConfigFileName is a constructor function that creates a configuration yaml file name
// that returns ConfigFileName.
func NewConfigFileName(dirName string, params ...string) file.Name {
	fileName := strings.Join(params, "-")

	fileNameFull := fmt.Sprintf("%v/%v.yaml", dirName, fileName)

	return file.Name(fileNameFull)
}
