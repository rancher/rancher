package userstored

import (
	"context"
	"net/http"
	"time"

	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	namespacecustom "github.com/rancher/rancher/pkg/api/customization/namespace"
	"github.com/rancher/rancher/pkg/api/customization/persistentvolumeclaim"
	sec "github.com/rancher/rancher/pkg/api/customization/secret"
	"github.com/rancher/rancher/pkg/api/customization/yaml"
	"github.com/rancher/rancher/pkg/api/store/apiservice"
	"github.com/rancher/rancher/pkg/api/store/cert"
	"github.com/rancher/rancher/pkg/api/store/crd"
	"github.com/rancher/rancher/pkg/api/store/hpa"
	"github.com/rancher/rancher/pkg/api/store/ingress"
	"github.com/rancher/rancher/pkg/api/store/namespace"
	"github.com/rancher/rancher/pkg/api/store/nocondition"
	"github.com/rancher/rancher/pkg/api/store/pod"
	"github.com/rancher/rancher/pkg/api/store/projectsetter"
	"github.com/rancher/rancher/pkg/api/store/secret"
	"github.com/rancher/rancher/pkg/api/store/service"
	"github.com/rancher/rancher/pkg/api/store/storageclass"
	"github.com/rancher/rancher/pkg/api/store/workload"
	clusterClient "github.com/rancher/rancher/pkg/client/generated/cluster/v3"
	client "github.com/rancher/rancher/pkg/client/generated/project/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	clusterschema "github.com/rancher/rancher/pkg/schemas/cluster.cattle.io/v3"
	schema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

func Setup(ctx context.Context, mgmt *config.ScaledContext, clusterManager *clustermanager.Manager, k8sProxy http.Handler) error {
	// Here we setup all types that will be stored in the User cluster

	schemas := mgmt.Schemas

	addProxyStore(ctx, schemas, mgmt, client.ConfigMapType, "v1", nil)
	addProxyStore(ctx, schemas, mgmt, client.CronJobType, "batch/v1beta1", workload.NewCustomizeStore)
	addProxyStore(ctx, schemas, mgmt, client.DaemonSetType, "apps/v1", workload.NewCustomizeStore)
	addProxyStore(ctx, schemas, mgmt, client.DeploymentType, "apps/v1", workload.NewCustomizeStore)
	//TODO: Update to use networking.k8s.io/v1beta1 when k8s 1.3 is no longer supported
	addProxyStore(ctx, schemas, mgmt, client.IngressType, "extensions/v1beta1", ingress.Wrap)
	addProxyStore(ctx, schemas, mgmt, client.JobType, "batch/v1", workload.NewCustomizeStore)
	addProxyStore(ctx, schemas, mgmt, client.PersistentVolumeClaimType, "v1", nil)
	addProxyStore(ctx, schemas, mgmt, client.PodType, "v1", func(store types.Store) types.Store {
		return pod.New(store, clusterManager, mgmt)
	})
	addProxyStore(ctx, schemas, mgmt, client.ReplicaSetType, "apps/v1", workload.NewCustomizeStore)
	addProxyStore(ctx, schemas, mgmt, client.ReplicationControllerType, "v1", workload.NewCustomizeStore)
	addProxyStore(ctx, schemas, mgmt, client.ServiceType, "v1", service.New)
	addProxyStore(ctx, schemas, mgmt, client.StatefulSetType, "apps/v1", workload.NewCustomizeStore)
	addProxyStore(ctx, schemas, mgmt, clusterClient.NamespaceType, "v1", namespace.New)
	addProxyStore(ctx, schemas, mgmt, clusterClient.PersistentVolumeType, "v1", nil)
	addProxyStore(ctx, schemas, mgmt, clusterClient.APIServiceType, "apiregistration.k8s.io/v1", nil)
	addProxyStore(ctx, schemas, mgmt, client.HorizontalPodAutoscalerType, "autoscaling/v2beta2", nil)
	addProxyStore(ctx, schemas, mgmt, clusterClient.StorageClassType, "storage.k8s.io/v1", nil)
	addProxyStore(ctx, schemas, mgmt, client.PrometheusType, "monitoring.coreos.com/v1", nil)
	addProxyStore(ctx, schemas, mgmt, client.PrometheusRuleType, "monitoring.coreos.com/v1", nil)
	addProxyStore(ctx, schemas, mgmt, client.AlertmanagerType, "monitoring.coreos.com/v1", nil)
	addProxyStore(ctx, schemas, mgmt, client.ServiceMonitorType, "monitoring.coreos.com/v1", nil)
	addProxyStore(ctx, schemas, mgmt, client.VirtualServiceType, "networking.istio.io/v1alpha3", nil)
	addProxyStore(ctx, schemas, mgmt, client.DestinationRuleType, "networking.istio.io/v1alpha3", nil)
	addProxyStore(ctx, schemas, mgmt, client.GatewayType, "networking.istio.io/v1alpha3", nil)

	Secret(ctx, mgmt, schemas)
	Service(ctx, schemas, mgmt)
	Workload(schemas, clusterManager)
	Namespace(schemas, clusterManager)
	HPA(schemas, clusterManager)
	Istio(schemas)

	SetProjectID(schemas, clusterManager, k8sProxy)
	StorageClass(schemas)
	PersistentVolumeClaim(clusterManager, schemas)

	return nil
}

func SetProjectID(schemas *types.Schemas, clusterManager *clustermanager.Manager, k8sProxy http.Handler) {
	for _, schema := range schemas.SchemasForVersion(schema.Version) {
		if schema.Store == nil || schema.Store.Context() != config.UserStorageContext {
			continue
		}

		if schema.CanList(nil) != nil {
			continue
		}

		if _, ok := schema.ResourceFields["namespaceId"]; !ok {
			panic(schema.ID + " does not have namespaceId")
		}

		if _, ok := schema.ResourceFields["projectId"]; !ok {
			panic(schema.ID + " does not have projectId")
		}

		schema.Store = projectsetter.New(schema.Store, clusterManager)
		schema.Formatter = yaml.NewFormatter(schema.Formatter)
		schema.LinkHandler = yaml.NewLinkHandler(k8sProxy, clusterManager, schema.LinkHandler)
	}
}

func StorageClass(schemas *types.Schemas) {
	storageClassSchema := schemas.Schema(&clusterschema.Version, "storageClass")
	storageClassSchema.Store = storageclass.Wrap(storageClassSchema.Store)
}

func PersistentVolumeClaim(cmanager *clustermanager.Manager, schemas *types.Schemas) {
	pvcSchema := schemas.Schema(&schema.Version, "persistentVolumeClaim")

	v := persistentvolumeclaim.Validator{
		ClusterManager: cmanager,
	}
	pvcSchema.Validator = v.Validator
}

func Namespace(schemas *types.Schemas, manager *clustermanager.Manager) {
	namespaceSchema := schemas.Schema(&clusterschema.Version, "namespace")
	namespaceSchema.LinkHandler = namespacecustom.NewLinkHandler(namespaceSchema.LinkHandler, manager)
	namespaceSchema.Formatter = namespacecustom.NewFormatter(yaml.NewFormatter(namespaceSchema.Formatter))
	actionWrapper := namespacecustom.ActionWrapper{
		ClusterManager: manager,
	}
	namespaceSchema.ActionHandler = actionWrapper.ActionHandler
}

func Workload(schemas *types.Schemas, clusterManager *clustermanager.Manager) {
	workload.NewWorkloadAggregateStore(schemas, clusterManager)
}

func Service(ctx context.Context, schemas *types.Schemas, mgmt *config.ScaledContext) {
	serviceSchema := schemas.Schema(&schema.Version, "service")
	dnsSchema := schemas.Schema(&schema.Version, "dnsRecord")
	// Move service store to DNSRecord and create new store on service, so they are then
	// same store but two different instances
	dnsSchema.Store = serviceSchema.Store
	addProxyStore(ctx, schemas, mgmt, client.ServiceType, "v1", service.New)
}

func Secret(ctx context.Context, management *config.ScaledContext, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, "namespacedSecret")
	schema.Store = secret.NewNamespacedSecretStore(ctx, management.ClientGetter)
	schema.Validator = sec.Validator

	for _, subSchema := range schemas.Schemas() {
		if subSchema.BaseType == schema.ID && subSchema.ID != schema.ID {
			subSchema.Store = subtype.NewSubTypeStore(subSchema.ID, schema.Store)
			subSchema.Validator = sec.Validator
		}
	}

	schema = schemas.Schema(&schema.Version, "namespacedCertificate")
	schema.Store = cert.Wrap(schema.Store)
}

func Istio(schemas *types.Schemas) {
	istioTypes := []string{client.VirtualServiceType, client.DestinationRuleType, client.GatewayType}
	for _, t := range istioTypes {
		schema := schemas.Schema(&schema.Version, t)
		store := &crd.ForgetCRDNotFoundStore{
			Store: schema.Store,
		}
		schema.Store = store
	}

}

func HPA(schemas *types.Schemas, manager *clustermanager.Manager) {
	schema := schemas.Schema(&schema.Version, client.HorizontalPodAutoscalerType)
	schema.Store = apiservice.NewAPIServicFilterStoreFunc(manager, "autoscaling/v2beta2")(schema.Store)
	schema.Store = nocondition.NewWrapper("initializing", "")(schema.Store)
	schema.Store = hpa.NewIgnoreTransitioningErrorStore(schema.Store, 60*time.Second, "initializing")
}
