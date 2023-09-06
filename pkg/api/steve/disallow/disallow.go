package disallow

import (
	"net/http"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
	schema2 "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
)

// The resource names must be plural.
var (
	// AllowAll is a set of resources for which Rancher doesn't require authentication to perform any operation.
	AllowAll = map[string]bool{
		"clusterroletemplatebindings":                true,
		"globalrolebindings":                         true,
		"globalroles":                                true,
		"podsecurityadmissionconfigurationtemplates": true,
		"projects":                                   true,
		"projectroletemplatebindings":                true,
	}
	allowPost = map[string]bool{
		"settings": true,
	}
	allowPut = map[string]bool{
		"features": true,
		"settings": true,
	}
	disallowGet = map[string]bool{
		"preferences":               true,
		"sourcecodecredentials":     true,
		"sourcecodeproviderconfigs": true,
		"sourcecoderepositories":    true,
		"templatecontents":          true,
		"templates":                 true,
		"templateversions":          true,
		"tokens":                    true,
	}
)

func Register(server *steve.Server) {
	server.SchemaFactory.AddTemplate(schema2.Template{
		Customize: func(schema *types.APISchema) {
			gr := attributes.GR(schema)
			if gr.Group == "management.cattle.io" || gr.Group == "project.cattle.io" {
				if AllowAll[gr.Resource] {
					return
				}
				attributes.AddDisallowMethods(schema,
					http.MethodPatch,
					http.MethodDelete)
				if !allowPut[gr.Resource] {
					attributes.AddDisallowMethods(schema, http.MethodPut)
				}
				if !allowPost[gr.Resource] {
					attributes.AddDisallowMethods(schema, http.MethodPost)
				}
				if disallowGet[gr.Resource] {
					attributes.AddDisallowMethods(schema, http.MethodGet)
				}
			}
		},
	})
}
