package yaml

import (
	"github.com/rancher/norman/types"
)

func NewFormatter(next types.Formatter) types.Formatter {
	return func(request *types.APIContext, resource *types.RawResource) {
		if next != nil {
			next(request, resource)
		}
		resource.Links["yaml"] = request.URLBuilder.Link("yaml", resource)
	}
}
