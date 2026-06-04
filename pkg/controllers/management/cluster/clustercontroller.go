package cluster
import (
	"context"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	GoogleCloudLoadBalancer = "GCLB"
	ElasticLoadBalancer     = "ELB"
	AzureL4LB               = "Azure L4 LB"
	NginxIngressProvider    = "Nginx"
)

type controller struct {
	clusterClient         v3.ClusterInterface
	nodeLister            v3.NodeLister
	kontainerDriverLister v3.KontainerDriverLister
	namespaces            v1.NamespaceInterface
	coreV1                v1.Interface
}

func Register(ctx context.Context, management *config.ManagementContext) {
	c := controller{
		clusterClient:         management.Management.Clusters(""),
		nodeLister:            management.Management.Nodes("").Controller().Lister(),
		kontainerDriverLister: management.Management.KontainerDrivers("").Controller().Lister(),
		namespaces:            management.Core.Namespaces(""),
		coreV1:                management.Core,
	}

	c.clusterClient.AddHandler(ctx, "clusterCreateUpdate", c.capsSync)
}
func (c *controller) capsSync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}
	if cluster.Spec.ImportedConfig != nil {
		return nil, nil
	}
	return nil, nil
}
