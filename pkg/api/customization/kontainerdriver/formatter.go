package kontainerdriver

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/types/client/management/v3"
)

func Formatter(request *types.APIContext, resource *types.RawResource) {
	state, ok := resource.Values["state"].(string)
	if ok {
		if state == "active" {
			resource.AddAction(request, "deactivate")
		}

		if state == "inactive" {
			resource.AddAction(request, "activate")
		}
	}

	if builtIn, _ := resource.Values[client.KontainerDriverFieldBuiltIn].(bool); builtIn {
		delete(resource.Links, "remove")
	}
}
