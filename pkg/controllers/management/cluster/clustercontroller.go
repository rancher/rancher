package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	rketypes "github.com/rancher/rke/types"

	errorsutil "github.com/pkg/errors"
	normantypes "github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	client "github.com/rancher/rancher/pkg/client/generated/project/v3"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/service"
	"github.com/rancher/rancher/pkg/kontainer-engine/types"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rke/cloudprovider/aws"
	"github.com/rancher/rke/cloudprovider/azure"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	GoogleCloudLoadBalancer = "GCLB"
	ElasticLoadBalancer     = "ELB"
	AzureL4LB               = "Azure L4 LB"
	NginxIngressProvider    = "Nginx"
	DefaultNodePortRange    = "30000-32767"
	capabilitiesAnnotation  = "capabilities/"
)

type controller struct {
	clusterClient         v3.ClusterInterface
	nodeLister            v3.NodeLister
	kontainerDriverLister v3.KontainerDriverLister
	namespaces            v1.NamespaceInterface
	coreV1                v1.Interface
	capabilitiesSchema    *normantypes.Schema
}

func Register(ctx context.Context, management *config.ManagementContext) {
	c := controller{
		clusterClient:         management.Management.Clusters(""),
		nodeLister:            management.Management.Nodes("").Controller().Lister(),
		kontainerDriverLister: management.Management.KontainerDrivers("").Controller().Lister(),
		namespaces:            management.Core.Namespaces(""),
		coreV1:                management.Core,
		capabilitiesSchema:    management.Schemas.Schema(&managementschema.Version, client.CapabilitiesType).InternalSchema,
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
	capabilities := v32.Capabilities{}

	if cluster.Spec.RancherKubernetesEngineConfig != nil {
		capabilities.NodePortRange = DefaultNodePortRange
		// taint support capability is set in provisioner and update cluster is called, so we should retain the capability here
		if cluster.Status.Capabilities.TaintSupport != nil && *cluster.Status.Capabilities.TaintSupport {
			supportsTaints := true
			capabilities.TaintSupport = &supportsTaints
		}
		if capabilities, err = c.RKECapabilities(capabilities, *cluster.Spec.RancherKubernetesEngineConfig, cluster.Name); err != nil {
			return nil, err
		}
	} else if cluster.Spec.GenericEngineConfig != nil {
		capabilities.NodePortRange = DefaultNodePortRange
		driverName, ok := (*cluster.Spec.GenericEngineConfig)["driverName"].(string)
		if !ok {
			logrus.Warnf("cluster %v had generic engine config but no driver name, k8s capabilities will "+
				"not be populated correctly", key)
			return nil, nil
		}

		kontainerDriver, err := c.kontainerDriverLister.Get("", driverName)
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, errorsutil.WithMessage(err, fmt.Sprintf("error getting kontainer driver: %v", driverName))
			}
			//do not return not found errors since the driver may have been deleted
			return nil, nil
		}

		driver := service.NewEngineService(
			clusterprovisioner.NewPersistentStore(c.namespaces, c.coreV1, c.clusterClient),
		)
		k8sCapabilities, err := driver.GetK8sCapabilities(context.Background(), kontainerDriver.Name, kontainerDriver,
			cluster.Spec)
		if err != nil {
			return nil, fmt.Errorf("error getting k8s capabilities: %v", err)
		}

		capabilities = toCapabilities(k8sCapabilities)
	}

	capabilities, err = c.overrideCapabilities(cluster.Annotations, capabilities)
	if err != nil {
		return nil, err
	}

	if !reflect.DeepEqual(capabilities, cluster.Status.Capabilities) {
		toUpdateCluster := cluster.DeepCopy()
		toUpdateCluster.Status.Capabilities = capabilities
		if _, err := c.clusterClient.Update(toUpdateCluster); err != nil {
			return nil, err
		}
		return toUpdateCluster, nil
	}

	return nil, nil
}

// overrideCapabilities masks the given Capabilities struct with values extracted from annotations prefixed with
// "capabilities.cattle.io/"
func (c *controller) overrideCapabilities(annotations map[string]string, oldCapabilities v32.Capabilities) (v32.Capabilities, error) {
	capabilities := v32.Capabilities{}

	capabilitiesMap, err := convert.EncodeToMap(oldCapabilities)
	if err != nil {
		return capabilities, err
	}

	var isUpdate bool
	for annoKey, annoValue := range annotations {
		if strings.HasPrefix(annoKey, capabilitiesAnnotation) {
			capability := strings.TrimPrefix(annoKey, capabilitiesAnnotation)
			val, err := c.parseResourceInterface(capability, annoValue)
			if err != nil {
				return capabilities, err
			}
			capabilitiesMap[capability] = val
			isUpdate = true
		}

	}

	if isUpdate {
		if err := convert.ToObj(capabilitiesMap, &capabilities); err != nil {
			return capabilities, err
		}
		return capabilities, nil
	}

	return oldCapabilities, nil
}

// parseResourceInterface converts a capability annotation to the appropriate type based on given schema.
// Json format is assumed if type is not bool, string, or integer.
func (c *controller) parseResourceInterface(key string, annoValue string) (interface{}, error) {
	resourceField, ok := c.capabilitiesSchema.ResourceFields[key]
	if resourceField.Nullable && annoValue == "" {
		return nil, nil
	}

	if !ok {
		return nil, fmt.Errorf("resource field [%s] from capabillities annotation not found", key)
	}

	fieldType := c.capabilitiesSchema.ResourceFields[key].Type
	switch fieldType {
	case "string":
		return annoValue, nil
	case "boolean":
		return strconv.ParseBool(annoValue)
	case "integer":
		return strconv.Atoi(annoValue)
	default:
		var result interface{}

		err := json.Unmarshal([]byte(annoValue), &result)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
}

func (c *controller) RKECapabilities(capabilities v32.Capabilities, rkeConfig rketypes.RancherKubernetesEngineConfig, clusterName string) (v32.Capabilities, error) {
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

	ingressController := c.IngressCapability(true, rkeConfig.Ingress.Provider)
	capabilities.IngressCapabilities = []v32.IngressCapabilities{ingressController}
	if rkeConfig.Services.KubeAPI.ServiceNodePortRange != "" {
		capabilities.NodePortRange = rkeConfig.Services.KubeAPI.ServiceNodePortRange
	} else if rkeConfig.Services.KubeAPI.ExtraArgs["service-node-port-range"] != "" {
		capabilities.NodePortRange = rkeConfig.Services.KubeAPI.ExtraArgs["service-node-port-range"]
	}

	return capabilities, nil
}

func (c *controller) L4Capability(enabled bool, providerName string, protocols []string, healthCheck bool) v32.LoadBalancerCapabilities {
	l4lb := v32.LoadBalancerCapabilities{
		Enabled:              &enabled,
		Provider:             providerName,
		ProtocolsSupported:   protocols,
		HealthCheckSupported: healthCheck,
	}
	return l4lb
}

func (c *controller) IngressCapability(httpLBEnabled bool, providerName string) v32.IngressCapabilities {
	customDefaultBackendDisabled := false
	ing := v32.IngressCapabilities{
		IngressProvider: providerName,
	}
	if strings.EqualFold(providerName, NginxIngressProvider) {
		ing.CustomDefaultBackend = &customDefaultBackendDisabled
	}
	return ing
}

func toCapabilities(k8sCapabilities *types.K8SCapabilities) v32.Capabilities {
	var controllers []v32.IngressCapabilities

	for _, controller := range k8sCapabilities.IngressControllers {
		controllers = append(controllers, v32.IngressCapabilities{
			CustomDefaultBackend: &controller.CustomDefaultBackend,
			IngressProvider:      controller.IngressProvider,
		})
	}

	return v32.Capabilities{
		IngressCapabilities: controllers,
		LoadBalancerCapabilities: v32.LoadBalancerCapabilities{
			Enabled:              &k8sCapabilities.L4LoadBalancer.Enabled,
			HealthCheckSupported: k8sCapabilities.L4LoadBalancer.HealthCheckSupported,
			ProtocolsSupported:   k8sCapabilities.L4LoadBalancer.ProtocolsSupported,
			Provider:             k8sCapabilities.L4LoadBalancer.Provider,
		},
		NodePoolScalingSupported: k8sCapabilities.NodePoolScalingSupported,
		NodePortRange:            k8sCapabilities.NodePortRange,
	}
}
