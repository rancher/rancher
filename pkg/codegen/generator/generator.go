package generator

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"net/http"
	"os"
	"path/filepath"
	"text/template"

	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	outputDir   = "./pkg/generated"
	basePackage = "github.com/rancher/rancher/pkg/apis"
	baseCattle  = "../client/generated"
	baseK8s     = "norman"
	baseCompose = "compose"
)

func funcs() template.FuncMap {
	return template.FuncMap{
		"capitalize":   convert.Capitalize,
		"unCapitalize": convert.Uncapitalize,
		"upper":        strings.ToUpper,
		"toLower":      strings.ToLower,
		"hasGet":       hasGet,
		"hasPost":      hasPost,
	}
}

func hasGet(schema *types.Schema) bool {
	return contains(schema.CollectionMethods, http.MethodGet)
}

func hasPost(schema *types.Schema) bool {
	return contains(schema.CollectionMethods, http.MethodPost)
}

func contains(list []string, needle string) bool {
	return slices.Contains(list, needle)
}

func Generate(schemas *types.Schemas, backendTypes map[string]bool) {
	version := getVersion(schemas)
	group := strings.Split(version.Group, ".")[0]

	cattleOutputPackage := path.Join(baseCattle, group, version.Version)
	k8sOutputPackage := path.Join(baseK8s, version.Group, version.Version)

	if err := generator.Generate(schemas, backendTypes, basePackage, outputDir, cattleOutputPackage, k8sOutputPackage); err != nil {
		panic(err)
	}
}

func GenerateClient(schemas *types.Schemas, backendTypes map[string]bool) {
	version := getVersion(schemas)
	group := strings.Split(version.Group, ".")[0]

	cattleOutputPackage := path.Join(baseCattle, group, version.Version)

	if err := generator.GenerateClient(schemas, backendTypes, outputDir, cattleOutputPackage); err != nil {
		panic(err)
	}
}

func GenerateComposeType(projectSchemas *types.Schemas, managementSchemas *types.Schemas, clusterSchemas *types.Schemas) {
	if err := generateComposeType(filepath.Join(outputDir, baseCompose), projectSchemas, managementSchemas, clusterSchemas); err != nil {
		panic(err)
	}
}

func generateComposeType(baseCompose string, projectSchemas *types.Schemas, managementSchemas *types.Schemas, clusterSchemas *types.Schemas) error {
	outputDir := filepath.Join(defaultSourceTree(), baseCompose)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	filePath := "zz_generated_compose.go"
	output, err := os.Create(path.Join(outputDir, filePath))
	if err != nil {
		return err
	}
	defer output.Close()

	typeTemplate, err := template.New("compose.template").
		Funcs(funcs()).
		Parse(strings.Replace(composeTemplate, "%BACK%", "`", -1))
	if err != nil {
		return err
	}

	if err := typeTemplate.Execute(output, map[string]interface{}{
		"managementSchemas": managementSchemas.Schemas(),
		"projectSchemas":    projectSchemas.Schemas(),
		"clusterSchemas":    clusterSchemas.Schemas(),
	}); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}

	return generator.Gofmt(defaultSourceTree(), baseCompose)
}

func GenerateNativeTypes(gv schema.GroupVersion, nsObjs []interface{}, objs []interface{}) {
	version := gv.Version
	group := gv.Group
	groupPath := group

	if groupPath == "" {
		groupPath = "core"
	}

	k8sOutputPackage := path.Join(outputDir, baseK8s, groupPath, version)

	if err := generator.GenerateControllerForTypes(&types.APIVersion{
		Version: version,
		Group:   group,
		Path:    fmt.Sprintf("/k8s/%s-%s", groupPath, version),
	}, k8sOutputPackage, nsObjs, objs); err != nil {
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

// From gengo/args, v1:
// defaultSourceTree returns the /src directory of the first entry in $GOPATH.
// If $GOPATH is empty, it returns "./". Useful as a default output location.
func defaultSourceTree() string {
	paths := strings.Split(os.Getenv("GOPATH"), string(filepath.ListSeparator))
	if len(paths) > 0 && len(paths[0]) > 0 {
		return filepath.Join(paths[0], "src")
	}
	return "./"
}
