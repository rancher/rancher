package setup

import (
	"context"

	"github.com/rancher/norman/pkg/subscribe"
	"github.com/rancher/norman/store/crd"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/store/subtype"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/user/api/pod"
	"github.com/rancher/rancher/pkg/api/user/api/workload"
	"github.com/rancher/rancher/pkg/api/user/store/ingress"
	"github.com/rancher/rancher/pkg/api/user/store/secret"
	clusterSchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"k8s.io/client-go/rest"
)

func Schemas(ctx context.Context, app *config.UserContext, schemas *types.Schemas) error {
	subscribe.Register(&clusterSchema.Version, schemas)
	subscribe.Register(&schema.Version, schemas)
	DaemonSet(app.UnversionedClient, schemas)
	Deployment(app.UnversionedClient, schemas)
	Ingress(app.UserOnlyContext(), schemas)
	Namespace(app.UnversionedClient, schemas)
	Node(app.UnversionedClient, schemas)
	PersistentVolume(app.UnversionedClient, schemas)
	PersistentVolumeClaims(app.UnversionedClient, schemas)
	Pod(app.UnversionedClient, schemas)
	ReplicaSet(app.UnversionedClient, schemas)
	ReplicationController(app.UnversionedClient, schemas)
	Secret(app.UnversionedClient, schemas)
	Service(app.UnversionedClient, schemas)
	StatefulSet(app.UnversionedClient, schemas)

	crdStore, err := crd.NewCRDStoreFromConfig(app.RESTConfig)
	if err != nil {
		return err
	}

	if err := crdStore.AddSchemas(ctx, schemas.Schema(&schema.Version, client.WorkloadType)); err != nil {
		return err
	}

	// After CRD store is set on workload
	Workload(schemas)

	return nil
}

func Namespace(k8sClient rest.Interface, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, "namespace")
	schema.Store = &transform.Store{
		Store: proxy.NewProxyStore(k8sClient,
			[]string{"api"},
			"",
			"v1",
			"Namespace",
			"namespaces"),
		Transformer: func(apiContext *types.APIContext, data map[string]interface{}) (map[string]interface{}, error) {
			anns, _ := data["annotations"].(map[string]interface{})
			if anns["management.cattle.io/system-namespace"] == "true" {
				return nil, nil
			}
			return data, nil
		},
	}

	clusterSchema := schemas.Schema(&clusterSchema.Version, "namespace")
	clusterSchema.Store = schema.Store
}

func Node(k8sClient rest.Interface, schemas *types.Schemas) {
	schema := schemas.Schema(&clusterSchema.Version, "node")
	schema.Store = proxy.NewProxyStore(k8sClient,
		[]string{"api"},
		"",
		"v1",
		"Node",
		"nodes")
}

func PersistentVolume(k8sClient rest.Interface, schemas *types.Schemas) {
	schema := schemas.Schema(&clusterSchema.Version, "persistentVolume")
	schema.Store = proxy.NewProxyStore(k8sClient,
		[]string{"api"},
		"",
		"v1",
		"PersistentVolume",
		"persistentvolumes")
}

func PersistentVolumeClaims(k8sClient rest.Interface, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, "persistentVolumeClaim")
	schema.Store = proxy.NewProxyStore(k8sClient,
		[]string{"api"},
		"",
		"v1",
		"PersistentVolumeClaim",
		"persistentvolumeclaims")
}

func DaemonSet(k8sClient rest.Interface, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, "daemonSet")
	schema.Store = &workload.PrefixTypeStore{
		Store: proxy.NewProxyStore(k8sClient,
			[]string{"apis"},
			"apps",
			"v1beta2",
			"DaemonSet",
			"daemonsets"),
	}
}

func ReplicaSet(k8sClient rest.Interface, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, "replicaSet")
	schema.Store = &workload.PrefixTypeStore{
		Store: proxy.NewProxyStore(k8sClient,
			[]string{"apis"},
			"apps",
			"v1beta2",
			"ReplicaSet",
			"replicasets"),
	}
}

func ReplicationController(k8sClient rest.Interface, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, "replicationController")
	schema.Store = &workload.PrefixTypeStore{
		Store: proxy.NewProxyStore(k8sClient,
			[]string{"api"},
			"",
			"v1",
			"ReplicationController",
			"replicationcontrollers"),
	}
}

func Deployment(k8sClient rest.Interface, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, "deployment")
	schema.Store = &workload.PrefixTypeStore{
		Store: proxy.NewProxyStore(k8sClient,
			[]string{"apis"},
			"apps",
			"v1beta2",
			"Deployment",
			"deployments"),
	}
}

func Workload(schemas *types.Schemas) {
	workload.ConfigureStore(schemas)
}

func StatefulSet(k8sClient rest.Interface, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, "statefulSet")
	schema.Store = &workload.PrefixTypeStore{
		Store: proxy.NewProxyStore(k8sClient,
			[]string{"apis"},
			"apps",
			"v1beta2",
			"StatefulSet",
			"statefulsets"),
	}
}

func Service(k8sClient rest.Interface, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, "dnsRecord")
	schema.Store = proxy.NewProxyStore(k8sClient,
		[]string{"api"},
		"",
		"v1",
		"Service",
		"services")

	serviceSchema := schemas.Schema(&schema.Version, "service")
	serviceSchema.Store = schema.Store
}

func Ingress(workload *config.UserOnlyContext, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, "ingress")
	schema.Store = ingress.NewStore(workload)
}

func Secret(k8sClient rest.Interface, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, "namespacedSecret")
	schema.Store = secret.NewSecretStore(k8sClient, schemas)

	for _, subSchema := range schemas.Schemas() {
		if subSchema.BaseType == "secret" && subSchema.ID != "namespacedSecret" && subSchema.ID != "secret" {
			subSchema.Store = subtype.NewSubTypeStore(subSchema.ID, schema.Store)
		}
	}
}

func Pod(k8sClient rest.Interface, schemas *types.Schemas) {
	schema := schemas.Schema(&schema.Version, client.PodType)
	schema.Store = &transform.Store{
		Store: proxy.NewProxyStore(k8sClient,
			[]string{"api"},
			"",
			"v1",
			"Pod",
			"pods"),
		Transformer:       pod.Transform,
		ListTransformer:   pod.ListTransform,
		StreamTransformer: pod.StreamTransform,
	}
}
