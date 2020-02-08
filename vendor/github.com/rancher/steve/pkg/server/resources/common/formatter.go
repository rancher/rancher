package common

import (
	"github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/steve/pkg/server/store/proxy"
	"k8s.io/apimachinery/pkg/api/meta"
)

func DefaultTemplate(clientGetter proxy.ClientGetter) schema.Template {
	return schema.Template{
		Store:     proxy.NewProxyStore(clientGetter),
		Formatter: Formatter,
	}
}

func Formatter(request *types.APIRequest, resource *types.RawResource) {
	meta, err := meta.Accessor(resource.APIObject.Object)
	if err != nil {
		return
	}

	selfLink := meta.GetSelfLink()
	if selfLink == "" {
		return
	}

	u := request.URLBuilder.RelativeToRoot(selfLink)
	resource.Links["view"] = u

	if _, ok := resource.Links["update"]; !ok {
		resource.Links["update"] = u
	}
}
