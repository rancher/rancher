package resources

import (
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/client"
	"github.com/rancher/steve/pkg/clustercache"
	"github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/schemaserver/store/apiroot"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/steve/pkg/server/resources/apigroups"
	"github.com/rancher/steve/pkg/server/resources/common"
	"github.com/rancher/steve/pkg/server/resources/counts"
	"k8s.io/client-go/discovery"
)

func DefaultSchemas(baseSchema *types.APISchemas, discovery discovery.DiscoveryInterface, ccache clustercache.ClusterCache) *types.APISchemas {
	counts.Register(baseSchema, ccache)
	apigroups.Register(baseSchema, discovery)
	apiroot.Register(baseSchema, []string{"v1"}, []string{"proxy:/apis"})
	return baseSchema
}

func DefaultSchemaTemplates(cf *client.Factory, lookup accesscontrol.AccessSetLookup) []schema.Template {
	return []schema.Template{
		common.DefaultTemplate(cf, lookup),
	}
}
