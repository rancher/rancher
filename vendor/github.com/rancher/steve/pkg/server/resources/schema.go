package resources

import (
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/client"
	"github.com/rancher/steve/pkg/clustercache"
	"github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/schemaserver/store/apiroot"
	"github.com/rancher/steve/pkg/schemaserver/subscribe"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/steve/pkg/server/resources/apigroups"
	"github.com/rancher/steve/pkg/server/resources/common"
	"github.com/rancher/steve/pkg/server/resources/counts"
	"github.com/rancher/steve/pkg/server/resources/userpreferences"
	"github.com/rancher/steve/pkg/server/store/proxy"
	"k8s.io/client-go/discovery"
)

func DefaultSchemas(baseSchema *types.APISchemas, ccache clustercache.ClusterCache, cg proxy.ClientGetter) *types.APISchemas {
	counts.Register(baseSchema, ccache)
	subscribe.Register(baseSchema)
	apiroot.Register(baseSchema, []string{"v1"}, []string{"proxy:/apis"})
	userpreferences.Register(baseSchema, cg)
	return baseSchema
}

func DefaultSchemaTemplates(cf *client.Factory, lookup accesscontrol.AccessSetLookup, discovery discovery.DiscoveryInterface) []schema.Template {
	return []schema.Template{
		common.DefaultTemplate(cf, lookup),
		apigroups.Template(discovery),
	}
}
