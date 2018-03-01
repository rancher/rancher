package clusteregistrationtokens

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

func Formatter(request *types.APIContext, resource *types.RawResource) {
	if convert.ToBool(resource.Values["internal"]) {
		delete(resource.Links, "remove")
	}
	resource.Links["shell"] = request.URLBuilder.Link("shell", resource)
}
