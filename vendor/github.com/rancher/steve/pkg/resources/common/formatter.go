package common

import (
	"strings"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/stores/proxy"
	"github.com/rancher/steve/pkg/summarycache"
	"github.com/rancher/wrangler/pkg/data"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func DefaultTemplate(clientGetter proxy.ClientGetter,
	summaryCache *summarycache.SummaryCache,
	asl accesscontrol.AccessSetLookup) schema.Template {
	return schema.Template{
		Store:     proxy.NewProxyStore(clientGetter, summaryCache, asl),
		Formatter: formatter(summaryCache),
	}
}

func formatter(summarycache *summarycache.SummaryCache) types.Formatter {
	return func(request *types.APIRequest, resource *types.RawResource) {
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
			summary, rel := summarycache.SummaryAndRelationship(unstr)
			data.PutValue(unstr.Object, map[string]interface{}{
				"name":          summary.State,
				"error":         summary.Error,
				"transitioning": summary.Transitioning,
				"message":       strings.Join(summary.Message, ":"),
			}, "metadata", "state")
			data.PutValue(unstr.Object, rel, "metadata", "relationships")
		}
	}
}
