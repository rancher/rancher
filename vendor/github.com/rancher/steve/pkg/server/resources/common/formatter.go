package common

import (
	"strings"

	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/steve/pkg/server/store/proxy"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/summary"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func DefaultTemplate(clientGetter proxy.ClientGetter, asl accesscontrol.AccessSetLookup) schema.Template {
	return schema.Template{
		Store:     proxy.NewProxyStore(clientGetter, asl),
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

	if unstr, ok := resource.APIObject.Object.(*unstructured.Unstructured); ok {
		summary := summary.Summarize(unstr)
		data.PutValue(unstr.Object, map[string]interface{}{
			"name":          summary.State,
			"error":         summary.Error,
			"transitioning": summary.Transitioning,
			"message":       strings.Join(summary.Message, ":"),
		}, "metadata", "state")
	}
}
