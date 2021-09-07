package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	clusterSchema "github.com/rancher/rancher/pkg/schemas/cluster.cattle.io/v3"
	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	publicSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3public"
	projectSchema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	"github.com/rancher/rancher/tests/automation-framework/codegen/generator"
)

func main() {
	os.Unsetenv("GOPATH")
	generator.GenerateClient(managementSchema.Schemas, map[string]bool{
		"userAttribute": true,
	})
	generator.GenerateClient(publicSchema.PublicSchemas, nil)
	generator.GenerateClient(clusterSchema.Schemas, map[string]bool{
		"clusterUserAttribute": true,
		"clusterAuthToken":     true,
	})
	generator.GenerateClient(projectSchema.Schemas, nil)

	if err := replaceClientBasePackager(); err != nil {
		panic(err)
	}
}

func replaceClientBasePackager() error {
	return filepath.Walk("./client/generated", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasPrefix(info.Name(), "zz_generated_client") {
			input, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			replacement := bytes.Replace(input, []byte("github.com/rancher/norman/clientbase"), []byte("github.com/rancher/rancher/tests/automation-framework/clientbase"), -1)

			if err = ioutil.WriteFile(path, replacement, 0666); err != nil {
				return err
			}
		}

		return nil
	})
}
