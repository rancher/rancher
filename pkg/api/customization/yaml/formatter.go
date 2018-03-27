package yaml

import (
	"github.com/rancher/norman/types"
)

func NewFormatter(next types.Formatter) types.Formatter {
	return func(request *types.APIContext, resource *types.RawResource) {
		resource.Links["yaml"] = request.URLBuilder.Link("yaml", resource)
		if next != nil {
			next(request, resource)
		}
	}
}
