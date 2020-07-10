package clusterconfigcopier

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

type controller struct {
}

func Register(ctx context.Context, management *config.ManagementContext) {
	c := controller{}

	management.Management.Clusters("").AddHandler(ctx, "clusterconfigcopier", c.sync)
}

func (c *controller) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if key == "" {
		return cluster, nil
	}

	if cluster.Spec.GenericEngineConfig != nil {
		return cluster, nil
	}

	if cluster.Spec.AmazonElasticContainerServiceConfig != nil {
		cluster.Spec.GenericEngineConfig = cluster.Spec.AmazonElasticContainerServiceConfig
		(*cluster.Spec.GenericEngineConfig)["driverName"] = "amazonelasticcontainerservice"
	}

	if cluster.Spec.AzureKubernetesServiceConfig != nil {
		cluster.Spec.GenericEngineConfig = cluster.Spec.AzureKubernetesServiceConfig
		(*cluster.Spec.GenericEngineConfig)["driverName"] = "azurekubernetesservice"
	}

	if cluster.Spec.GoogleKubernetesEngineConfig != nil {
		cluster.Spec.GenericEngineConfig = cluster.Spec.GoogleKubernetesEngineConfig
		(*cluster.Spec.GenericEngineConfig)["driverName"] = "googlekubernetesengine"
	}

	return nil, nil
}
