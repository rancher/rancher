package resources

import (
	"context"

	"github.com/rancher/apiserver/pkg/store/apiroot"
	"github.com/rancher/apiserver/pkg/subscribe"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/client"
	"github.com/rancher/steve/pkg/clustercache"
	"github.com/rancher/steve/pkg/resources/apigroups"
	"github.com/rancher/steve/pkg/resources/clusters"
	"github.com/rancher/steve/pkg/resources/common"
	"github.com/rancher/steve/pkg/resources/counts"
	"github.com/rancher/steve/pkg/resources/helm"
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
	helm.Register(baseSchema)

	err := clusters.Register(ctx, baseSchema, cg, ccache)
	return baseSchema, err
}

func DefaultSchemaTemplates(cf *client.Factory, lookup accesscontrol.AccessSetLookup, discovery discovery.DiscoveryInterface) []schema.Template {
	return []schema.Template{
		common.DefaultTemplate(cf, lookup),
		apigroups.Template(discovery),
		{
			ID:        "configmap",
			Formatter: common.DefaultFormatter(helm.DropHelmData),
		},
		{
			ID:        "secret",
			Formatter: common.DefaultFormatter(helm.DropHelmData),
		},
	}
}
