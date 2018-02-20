package userstored

import (
	"context"

	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/customization/workload"
	"github.com/rancher/rancher/pkg/api/store/ingress"
	"github.com/rancher/rancher/pkg/api/store/namespace"
	"github.com/rancher/rancher/pkg/api/store/pod"
	"github.com/rancher/rancher/pkg/api/store/projectsetter"
	"github.com/rancher/rancher/pkg/api/store/secret"
	"github.com/rancher/rancher/pkg/api/store/service"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	clusterClient "github.com/rancher/types/client/cluster/v3"
	"github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
)

func Setup(ctx context.Context, mgmt *config.ScaledContext) error {
	// Here we setup all types that will be stored in the User cluster

	schemas := mgmt.Schemas

	addProxyStore(schemas, mgmt, client.DaemonSetType, "apps/v1beta2", workload.New)
	addProxyStore(schemas, mgmt, client.DeploymentType, "apps/v1beta2", workload.New)
	addProxyStore(schemas, mgmt, client.PersistentVolumeClaimType, "v1", nil)
	addProxyStore(schemas, mgmt, client.PodType, "v1", pod.New)
	addProxyStore(schemas, mgmt, client.ReplicaSetType, "apps/v1beta2", workload.New)
	addProxyStore(schemas, mgmt, client.ReplicationControllerType, "v1", workload.New)
	addProxyStore(schemas, mgmt, client.ServiceType, "v1", service.New)
	addProxyStore(schemas, mgmt, client.StatefulSetType, "apps/v1beta2", nil)
	addProxyStore(schemas, mgmt, client.JobType, "batch/v1", workload.New)
	addProxyStore(schemas, mgmt, client.CronJobType, "batch/v1beta1", workload.New)
	addProxyStore(schemas, mgmt, clusterClient.NamespaceType, "v1", namespace.New)
	addProxyStore(schemas, mgmt, clusterClient.PersistentVolumeType, "v1", nil)
	addProxyStore(schemas, mgmt, client.IngressType, "extensions/v1beta1", ingress.Wrap)

	Secret(mgmt, schemas)
	Service(schemas)
	Workload(schemas)

	SetProjectID(schemas)

	return nil
}

func SetProjectID(schemas *types.Schemas) {
	for _, schema := range schemas.SchemasForVersion(schema.Version) {
		if schema.Store == nil || schema.Store.Context() != config.UserStorageContext {
			continue
		}

		if !schema.CanList(nil) {
			continue
		}

		if _, ok := schema.ResourceFields["namespaceId"]; !ok {
			panic(schema.ID + " does not have namespaceId")
		}

		if _, ok := schema.ResourceFields["projectId"]; !ok {
			panic(schema.ID + " does not have projectId")
		}

		schema.Store = projectsetter.Wrap(schema.Store)
	}
}

func Workload(schemas *types.Schemas) {
	workload.ConfigureStore(schemas)
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
}
