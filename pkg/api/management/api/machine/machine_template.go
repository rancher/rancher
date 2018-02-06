package machine

import "github.com/rancher/norman/types"

func TemplateFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	delete(resource.Values, "secretValues")
}
