package generator

import (
	"path"
	"strings"

	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/types"
)

var (
	outputDir  = "./"
	baseCattle = "./clients/rancher/generated"
)

func GenerateClient(schemas *types.Schemas, backendTypes map[string]bool) {
	version := getVersion(schemas)
	group := strings.Split(version.Group, ".")[0]

	cattleOutputPackage := path.Join(baseCattle, group, version.Version)

	if err := generator.GenerateClient(schemas, backendTypes, outputDir, cattleOutputPackage); err != nil {
		panic(err)
	}
}

func getVersion(schemas *types.Schemas) *types.APIVersion {
	var version types.APIVersion
	for _, schema := range schemas.Schemas() {
		if version.Group == "" {
			version = schema.Version
			continue
		}
		if version.Group != schema.Version.Group ||
			version.Version != schema.Version.Version {
			panic("schema set contains two APIVersions")
		}
	}

	return &version
}
