package resources

import (
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
	"github.com/rancher/wrangler/pkg/summary"
)

func Register(server *steve.Server) {
	server.SchemaFactory.AddTemplate(schema.Template{
		Group: "management.cattle.io",
		Kind:  "Feature",
		Formatter: func(request *types.APIRequest, resource *types.RawResource) {
			obj := resource.APIObject.Data()
			spec := obj.Map("spec")
			state := summary.Summary{
				State: "enabled",
			}
			if !getEffectiveValue(obj, spec) {
				state.State = "disabled"
			}
			resource.APIObject.Data().SetNested(state, "metadata", "state")
		},
	})
}

func getEffectiveValue(obj, spec map[string]interface{}) bool {
	if val := spec["value"]; val != nil {
		val, _ := val.(bool)
		return val
	}

	var val bool
	// if value is nil, then this ensure default value will be used
	status, ok := obj["status"].(map[string]interface{})
	if ok {
		val, _ = status["default"].(bool)
	}

	return val
}
