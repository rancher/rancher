package disallow

import (
	"context"
	"net/http"

	"github.com/rancher/steve/pkg/attributes"
	schema2 "github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/schemaserver/types"
	steve "github.com/rancher/steve/pkg/server"
)

var (
	allowPost = map[string]bool{
		"settings": true,
	}
	allowPut = map[string]bool{
		"features": true,
		"settings": true,
	}
	disallowGet = map[string]bool{
		"pipelines":                 true,
		"pipelineexecutions":        true,
		"pipelinesettings":          true,
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

type Server struct {
	ctx context.Context
}

func (s *Server) Setup(ctx context.Context, server *steve.Server) error {
	s.ctx = ctx
	server.SchemaTemplates = append(server.SchemaTemplates, schema2.Template{
		Customize: func(schema *types.APISchema) {
			gr := attributes.GR(schema)
			if gr.Group == "management.cattle.io" || gr.Group == "project.cattle.io" {
				attributes.AddDisallowMethods(schema,
					http.MethodPost,
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
	return nil
}
