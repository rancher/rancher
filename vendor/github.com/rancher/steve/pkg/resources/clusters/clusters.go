package clusters

import (
	"context"
	"net/http"
	"time"

	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/clustercache"
	"github.com/rancher/steve/pkg/stores/proxy"
	"github.com/rancher/steve/pkg/stores/switchschema"
	"github.com/rancher/steve/pkg/stores/switchstore"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
)

const (
	rancherCluster = "management.cattle.io.cluster"
	localID        = "local"
)

var (
	local = types.APIObject{
		Type:   "cluster",
		ID:     localID,
		Object: &Cluster{},
	}
	localList = types.APIObjectList{
		Objects: []types.APIObject{
			local,
		},
	}
)

func Register(ctx context.Context, schemas *types.APISchemas, cg proxy.ClientGetter, cluster clustercache.ClusterCache) error {
	k8s, err := cg.AdminK8sInterface()
	if err != nil {
		return err
	}

	shell := &shell{
		cg:        cg,
		namespace: "dashboard-shells",
	}

	picker := &picker{
		start:     time.Now(),
		discovery: k8s.Discovery(),
	}

	cluster.OnAdd(ctx, shell.PurgeOldShell)
	cluster.OnChange(ctx, func(gvr schema.GroupVersionResource, key string, obj, oldObj runtime.Object) error {
		return shell.PurgeOldShell(gvr, key, obj)
	})
	schemas.MustImportAndCustomize(Cluster{}, func(schema *types.APISchema) {
		schema.CollectionMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{http.MethodGet}
		schema.Formatter = Format
		schema.Store = &switchstore.Store{
			Picker: picker.Picker,
		}
		schema.LinkHandlers = map[string]http.Handler{
			"shell": shell,
		}
	})

	return nil
}

type Cluster struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec"`
	Status ClusterStatus `json:"status"`
}

type ClusterSpec struct {
	DisplayName string `json:"displayName"`
}

type ClusterStatus struct {
	Driver  string        `json:"driver"`
	Version *version.Info `json:"version,omitempty"`
}

func Format(request *types.APIRequest, resource *types.RawResource) {
	copy := [][]string{
		{"spec", "displayName"},
		{"metadata", "creationTimestamp"},
		{"status", "driver"},
		{"status", "version"},
	}

	from := resource.APIObject.Data()
	to := data.New()

	for _, keys := range copy {
		to.SetNested(data.GetValueN(from, keys...), keys...)
	}

	resource.APIObject.Object = to
	resource.Links["api"] = request.URLBuilder.RelativeToRoot("/k8s/clusters/" + resource.ID)
}

type Store struct {
	empty.Store

	start     time.Time
	discovery discovery.DiscoveryInterface
}

type picker struct {
	start     time.Time
	discovery discovery.DiscoveryInterface
}

func (p *picker) Picker(apiOp *types.APIRequest, schema *types.APISchema, verb, id string) (types.Store, error) {
	clusters := apiOp.Schemas.LookupSchema(rancherCluster)
	if clusters == nil {
		return &Store{
			start:     p.start,
			discovery: p.discovery,
		}, nil
	}
	return &switchschema.Store{
		Schema: clusters,
	}, nil
}

func (s *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	if id == localID {
		return s.newLocal(), nil
	}
	return types.APIObject{}, validation.NotFound
}

func (s *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	return types.APIObjectList{
		Objects: []types.APIObject{
			s.newLocal(),
		},
	}, nil
}

func (s *Store) newLocal() types.APIObject {
	cluster := &Cluster{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.NewTime(s.start),
		},
		Spec: ClusterSpec{
			DisplayName: "Remote",
		},
		Status: ClusterStatus{
			Driver: "remote",
		},
	}
	version, err := s.discovery.ServerVersion()
	if err == nil {
		cluster.Status.Version = version
	}
	return types.APIObject{
		Type:   "cluster",
		ID:     localID,
		Object: cluster,
	}
}
