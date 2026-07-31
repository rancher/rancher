package generator

import (
	"net/http"
	"os"
	"strings"
	"text/template"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

const (
	managementContextType = "mgmt"
)

func funcs() template.FuncMap {
	return template.FuncMap{
		"capitalize":          convert.Capitalize,
		"unCapitalize":        convert.Uncapitalize,
		"upper":               strings.ToUpper,
		"toLower":             strings.ToLower,
		"hasGet":              hasGet,
		"hasPost":             hasPost,
		"getCollectionOutput": getCollectionOutput,
		"namespaced":          namespaced,
	}
}

func addUnderscore(input string) string {
	return strings.ToLower(underscoreRegexp.ReplaceAllString(input, `${1}_${2}`))
}

func hasGet(schema *types.Schema) bool {
	return contains(schema.CollectionMethods, http.MethodGet)
}

func namespaced(schema *types.Schema) bool {
	return schema.Scope == types.NamespaceScope
}

func hasPost(schema *types.Schema) bool {
	return contains(schema.CollectionMethods, http.MethodPost)
}

func contains(list []string, needle string) bool {
	for _, i := range list {
		if i == needle {
			return true
		}
	}
	return false
}

func getCollectionOutput(output, codeName string) string {
	if output == "collection" {
		return codeName + "Collection"
	}
	return convert.Capitalize(output)
}

// SyncOnlyChangedObjects check whether the CATTLE_SKIP_NO_CHANGE_UPDATE env var is
// configured to skip the update handler for events on the management context
// that do not contain a change to the object.
func SyncOnlyChangedObjects() bool {
	skipNoChangeUpdate := os.Getenv("CATTLE_SYNC_ONLY_CHANGED_OBJECTS")
	if skipNoChangeUpdate == "" {
		return false
	}
	parts := strings.Split(skipNoChangeUpdate, ",")

	for _, part := range parts {
		if part == managementContextType {
			return true
		}
	}
	return false
}
