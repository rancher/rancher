package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/rancher/tests/framework/codegen/generator"
	managementSchema "github.com/rancher/rancher/tests/framework/pkg/schemas/management.cattle.io/v3"
)

func main() {
	os.Unsetenv("GOPATH")
	generator.GenerateClient(managementSchema.Schemas, map[string]bool{
		"userAttribute": true,
	})

	if err := replaceClientBasePackages(); err != nil {
		panic(err)
	}
}

// replaceClientBasePackages walks through the zz_generated_client genreated by generator.GenerateClient to replace imports from
// "github.com/rancher/norman/clientbase" to "github.com/rancher/rancher/tests/framework/pkg/clientbase" to use our modified code of the
// session.Session tracking the resources created by the Management Client.
func replaceClientBasePackages() error {
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
