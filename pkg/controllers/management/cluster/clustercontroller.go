package cluster

import (
	"context"
	"reflect"

	"github.com/rancher/rke/cloudprovider/aws"
	"github.com/rancher/rke/cloudprovider/azure"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	GoogleCloudLoadBalancer = "GCLB"
	ElasticLoadBalancer     = "ELB"
	AzureL4LB               = "Azure L4 LB"
	NginxIngressProvider    = "Nginx"
	DefaultNodePortRange    = "30000-32767"
)

type controller struct {
	clusterClient v3.ClusterInterface
	clusterLister v3.ClusterLister
	nodeLister    v3.NodeLister
}

func Register(ctx context.Context, management *config.ManagementContext) {
	c := controller{
		clusterClient: management.Management.Clusters(""),
		clusterLister: management.Management.Clusters("").Controller().Lister(),
		nodeLister:    management.Management.Nodes("").Controller().Lister(),
	}

	c.clusterClient.AddHandler(ctx, "clusterCreateUpdate", c.capsSync)
}

func (c *controller) capsSync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	var err error
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}

	if cluster.Spec.ImportedConfig != nil {
		return nil, nil
	}
	capabilities := v3.Capabilities{}
	capabilities.NodePortRange = DefaultNodePortRange

	if cluster.Spec.GoogleKubernetesEngineConfig != nil {
		capabilities = c.GKECapabilities(capabilities, *cluster.Spec.GoogleKubernetesEngineConfig)
	} else if cluster.Spec.AmazonElasticContainerServiceConfig != nil {
		capabilities = c.EKSCapabilities(capabilities, *cluster.Spec.AmazonElasticContainerServiceConfig)
	} else if cluster.Spec.AzureKubernetesServiceConfig != nil {
		capabilities = c.AKSCapabilities(capabilities, *cluster.Spec.AzureKubernetesServiceConfig)
	} else if cluster.Spec.RancherKubernetesEngineConfig != nil {
		if capabilities, err = c.RKECapabilities(capabilities, *cluster.Spec.RancherKubernetesEngineConfig, cluster.Name); err != nil {
			return nil, err
		}
	} else {
		return nil, nil
	}

	if !reflect.DeepEqual(capabilities, cluster.Status.Capabilities) {
		toUpdateCluster := cluster.DeepCopy()
		toUpdateCluster.Status.Capabilities = capabilities
		if _, err := c.clusterClient.Update(toUpdateCluster); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (c *controller) GKECapabilities(capabilities v3.Capabilities, gkeConfig v3.GoogleKubernetesEngineConfig) v3.Capabilities {
	capabilities.LoadBalancerCapabilities = c.L4Capability(true, GoogleCloudLoadBalancer, []string{"TCP", "UDP"}, true)
	if *gkeConfig.EnableHTTPLoadBalancing {
		ingressController := c.IngressCapability(true, GoogleCloudLoadBalancer, true)
		capabilities.IngressCapabilities = []v3.IngressCapabilities{ingressController}
	}
	return capabilities
}

func (c *controller) EKSCapabilities(capabilities v3.Capabilities, eksConfig v3.AmazonElasticContainerServiceConfig) v3.Capabilities {
	capabilities.LoadBalancerCapabilities = c.L4Capability(true, ElasticLoadBalancer, []string{"TCP"}, true)
	return capabilities
}

func (c *controller) AKSCapabilities(capabilities v3.Capabilities, aksConfig v3.AzureKubernetesServiceConfig) v3.Capabilities {
	capabilities.LoadBalancerCapabilities = c.L4Capability(true, AzureL4LB, []string{"TCP", "UDP"}, true)
	// on AKS portal you can enable Azure HTTP Application routing but Rancher doesn't have that option yet
	return capabilities
}

func (c *controller) RKECapabilities(capabilities v3.Capabilities, rkeConfig v3.RancherKubernetesEngineConfig, clusterName string) (v3.Capabilities, error) {
	switch rkeConfig.CloudProvider.Name {
	case aws.AWSCloudProviderName:
		capabilities.LoadBalancerCapabilities = c.L4Capability(true, ElasticLoadBalancer, []string{"TCP"}, true)
	case azure.AzureCloudProviderName:
		capabilities.LoadBalancerCapabilities = c.L4Capability(true, AzureL4LB, []string{"TCP", "UDP"}, true)
	}
	// only if not custom, non custom clusters have nodepools set
	nodes, err := c.nodeLister.List(clusterName, labels.Everything())
	if err != nil {
		return capabilities, err
	}

	if len(nodes) > 0 {
		if nodes[0].Spec.NodePoolName != "" {
			capabilities.NodePoolScalingSupported = true
		}
	}

	ingressController := c.IngressCapability(true, NginxIngressProvider, false)
	capabilities.IngressCapabilities = []v3.IngressCapabilities{ingressController}
	if rkeConfig.Services.KubeAPI.ExtraArgs["service-node-port-range"] != "" {
		capabilities.NodePortRange = rkeConfig.Services.KubeAPI.ExtraArgs["service-node-port-range"]
	}

	return capabilities, nil
}

func (c *controller) L4Capability(enabled bool, providerName string, protocols []string, healthCheck bool) v3.LoadBalancerCapabilities {
	l4lb := v3.LoadBalancerCapabilities{
		Enabled:              enabled,
		Provider:             providerName,
		ProtocolsSupported:   protocols,
		HealthCheckSupported: healthCheck,
	}
	return l4lb
}

func (c *controller) IngressCapability(httpLBEnabled bool, providerName string, customDefaultBackend bool) v3.IngressCapabilities {
	ing := v3.IngressCapabilities{
		IngressProvider:      providerName,
		CustomDefaultBackend: customDefaultBackend,
	}
	return ing
}
