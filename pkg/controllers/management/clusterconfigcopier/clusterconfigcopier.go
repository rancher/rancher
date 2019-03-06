package clusterconfigcopier

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/management/clusterconfigcensor"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

type controller struct {
	ConfigCensor *clusterconfigcensor.ConfigCensor
}

func Register(ctx context.Context, management *config.ManagementContext) {
	kontainerDriverLister := management.Management.KontainerDrivers("").Controller().Lister()
	dynamicSchemaLister := management.Management.DynamicSchemas("").Controller().Lister()

	c := controller{
		clusterconfigcensor.NewConfigCensor(kontainerDriverLister, dynamicSchemaLister),
	}

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

	censoredSpec, err := c.ConfigCensor.CensorGenericEngineConfig(cluster.Spec)
	if err != nil {
		return nil, err
	}

	// "Apply" the generic engine config spec
	cluster.Status.AppliedSpec.GenericEngineConfig = censoredSpec.GenericEngineConfig

	return cluster, nil
}
