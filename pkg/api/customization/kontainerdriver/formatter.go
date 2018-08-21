package kontainerdriver

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/types/client/management/v3"
)

func Formatter(request *types.APIContext, resource *types.RawResource) {
	resource.AddAction(request, "activate")
	resource.AddAction(request, "deactivate")

	if builtIn, _ := resource.Values[client.KontainerDriverFieldBuiltIn].(bool); builtIn {
		delete(resource.Links, "remove")
	}
}
