package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/codegen/generator"
)

func main() {
	os.Unsetenv("GOPATH")
	generator.GenerateClient(managementSchema.Schemas, map[string]bool{
		"userAttribute": true,
	})

	if err := replaceClientBasePackager(); err != nil {
		panic(err)
	}
}

func replaceClientBasePackager() error {
	return filepath.Walk("./clients/rancher/generated", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasPrefix(info.Name(), "zz_generated_client") {
			input, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			replacement := bytes.Replace(input, []byte("github.com/rancher/norman/clientbase"), []byte("github.com/rancher/rancher/tests/framework/pkg/clientbase"), -1)

			if err = ioutil.WriteFile(path, replacement, 0666); err != nil {
				return err
			}
		}

		return nil
	})
}
