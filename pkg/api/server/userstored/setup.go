package userstored

import (
	"context"
	"net/http"

	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	namespacecustom "github.com/rancher/rancher/pkg/api/customization/namespace"
	"github.com/rancher/rancher/pkg/api/customization/yaml"
	"github.com/rancher/rancher/pkg/api/store/cert"
	"github.com/rancher/rancher/pkg/api/store/ingress"
	"github.com/rancher/rancher/pkg/api/store/namespace"
	"github.com/rancher/rancher/pkg/api/store/pod"
	"github.com/rancher/rancher/pkg/api/store/projectsetter"
	"github.com/rancher/rancher/pkg/api/store/secret"
	"github.com/rancher/rancher/pkg/api/store/service"
	"github.com/rancher/rancher/pkg/api/store/workload"
	"github.com/rancher/rancher/pkg/clustermanager"
	clusterschema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	clusterClient "github.com/rancher/types/client/cluster/v3"
	"github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
)

func Setup(ctx context.Context, mgmt *config.ScaledContext, clusterManager *clustermanager.Manager, k8sProxy http.Handler) error {
	// Here we setup all types that will be stored in the User cluster

	schemas := mgmt.Schemas

	addProxyStore(schemas, mgmt, client.ConfigMapType, "v1", nil)
	addProxyStore(schemas, mgmt, client.CronJobType, "batch/v1beta1", workload.New)
	addProxyStore(schemas, mgmt, client.DaemonSetType, "apps/v1beta2", workload.New)
	addProxyStore(schemas, mgmt, client.DeploymentType, "apps/v1beta2", workload.New)
	addProxyStore(schemas, mgmt, client.IngressType, "extensions/v1beta1", ingress.Wrap)
	addProxyStore(schemas, mgmt, client.JobType, "batch/v1", workload.New)
	addProxyStore(schemas, mgmt, client.PersistentVolumeClaimType, "v1", nil)
	addProxyStore(schemas, mgmt, client.PodType, "v1", func(store types.Store) types.Store {
		return pod.New(store, clusterManager, mgmt)
	})
	addProxyStore(schemas, mgmt, client.ReplicaSetType, "apps/v1beta2", workload.New)
	addProxyStore(schemas, mgmt, client.ReplicationControllerType, "v1", workload.New)
	addProxyStore(schemas, mgmt, client.ServiceType, "v1", service.New)
	addProxyStore(schemas, mgmt, client.StatefulSetType, "apps/v1beta2", workload.New)
	addProxyStore(schemas, mgmt, clusterClient.NamespaceType, "v1", namespace.New)
	addProxyStore(schemas, mgmt, clusterClient.PersistentVolumeType, "v1", nil)
	addProxyStore(schemas, mgmt, clusterClient.StorageClassType, "storage.k8s.io/v1", nil)

	Secret(mgmt, schemas)
	Service(schemas)
	Workload(schemas, clusterManager)
	Namespace(schemas, clusterManager)

	SetProjectID(schemas, clusterManager, k8sProxy)

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

func Namespace(schemas *types.Schemas, manager *clustermanager.Manager) {
	namespaceSchema := schemas.Schema(&clusterschema.Version, "namespace")
	namespaceSchema.LinkHandler = namespacecustom.NewLinkHandler(namespaceSchema.LinkHandler, manager)
	namespaceSchema.Formatter = yaml.NewFormatter(namespaceSchema.Formatter)
	namespaceSchema.Validator = namespacecustom.Validator
}

func Workload(schemas *types.Schemas, clusterManager *clustermanager.Manager) {
	workload.ConfigureStore(schemas, clusterManager)
}

func Service(schemas *types.Schemas) {
	serviceSchema := schemas.Schema(&schema.Version, "service")
	dnsSchema := schemas.Schema(&schema.Version, "dnsRecord")
	dnsSchema.Store = serviceSchema.Store
}

func Secret(management *config.ScaledContext, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, "namespacedSecret")
	schema.Store = secret.NewNamespacedSecretStore(management.ClientGetter)

	for _, subSchema := range schemas.Schemas() {
		if subSchema.BaseType == schema.ID && subSchema.ID != schema.ID {
			subSchema.Store = subtype.NewSubTypeStore(subSchema.ID, schema.Store)
		}
	}

	schema = schemas.Schema(&schema.Version, "namespacedCertificate")
	schema.Store = cert.Wrap(schema.Store)
}
