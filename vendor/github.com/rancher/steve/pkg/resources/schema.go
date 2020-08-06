package resources

import (
	"context"

	"github.com/rancher/steve/pkg/summarycache"

	"github.com/rancher/apiserver/pkg/store/apiroot"
	"github.com/rancher/apiserver/pkg/subscribe"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/client"
	"github.com/rancher/steve/pkg/clustercache"
	"github.com/rancher/steve/pkg/resources/apigroups"
	"github.com/rancher/steve/pkg/resources/common"
	"github.com/rancher/steve/pkg/resources/counts"
	"github.com/rancher/steve/pkg/resources/formatters"
	"github.com/rancher/steve/pkg/resources/userpreferences"
	"github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/stores/proxy"
	"k8s.io/client-go/discovery"
)

func DefaultSchemas(ctx context.Context, baseSchema *types.APISchemas, ccache clustercache.ClusterCache, cg proxy.ClientGetter) (*types.APISchemas, error) {
	counts.Register(baseSchema, ccache)
	subscribe.Register(baseSchema)
	apiroot.Register(baseSchema, []string{"v1"}, "proxy:/apis")
	userpreferences.Register(baseSchema, cg)
	return baseSchema, nil
}

func DefaultSchemaTemplates(cf *client.Factory,
	summaryCache *summarycache.SummaryCache,
	lookup accesscontrol.AccessSetLookup,
	discovery discovery.DiscoveryInterface) []schema.Template {
	return []schema.Template{
		common.DefaultTemplate(cf, summaryCache, lookup),
		apigroups.Template(discovery),
		{
			ID:        "configmap",
			Formatter: formatters.DropHelmData,
		},
		{
			ID:        "secret",
			Formatter: formatters.DropHelmData,
		},
		{
			ID:        "pod",
			Formatter: formatters.Pod,
		},
	}
}
