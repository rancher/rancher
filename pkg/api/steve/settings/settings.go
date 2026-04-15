package settings

import (
	"fmt"
	"slices"

	"github.com/rancher/apiserver/pkg/types"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	schema2 "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
)

var ReadOnlySettings = []string{
	"cacerts",
}

func Register(server *steve.Server) {
	server.SchemaFactory.AddTemplate(schema2.Template{
		Group: "management.cattle.io",
		Kind:  "Setting",
		Formatter: func(request *types.APIRequest, resource *types.RawResource) {
			data := resource.APIObject.Data()
			if data.String("value") == "" {
				data.Set("value", data.String("default"))
			}

			if data.String("source") == "env" || slices.Contains(ReadOnlySettings, resource.ID) {
				delete(resource.Links, "update")
			} else {
				if err := request.AccessControl.CanDo(request, fmt.Sprintf("%s/%s", v3.SettingGroupVersionResource.Group, v3.SettingResource.Name), "update", resource.APIObject.Namespace(), resource.APIObject.Name()); err != nil {
					delete(resource.Links, "update")
				}
			}
		},
	})
}
