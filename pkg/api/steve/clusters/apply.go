package clusters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pborman/uuid"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
	steveschema "github.com/rancher/steve/pkg/schema"
	"github.com/rancher/steve/pkg/stores/proxy"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/yaml"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type Apply struct {
	cg            proxy.ClientGetter
	schemaFactory steveschema.Factory
}

func (a *Apply) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var (
		apiContext = types.GetAPIContext(req.Context())
		input      ApplyInput
	)

	if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
		apiContext.WriteError(err)
		return
	}

	objs, err := yaml.ToObjects(bytes.NewBufferString(input.YAML))
	if err != nil {
		apiContext.WriteError(err)
		return
	}

	apply, err := a.createApply(apiContext)
	if err != nil {
		apiContext.WriteError(err)
		return
	}

	if err := apply.WithDefaultNamespace(input.DefaultNamespace).ApplyObjects(objs...); err != nil {
		apiContext.WriteError(err)
		return
	}

	var result types.APIObjectList

	for _, obj := range objs {
		result.Objects = append(result.Objects, a.toAPIObject(apiContext, obj, input.DefaultNamespace))
	}

	apiContext.WriteResponseList(http.StatusOK, result)
}

func (a *Apply) toAPIObject(apiContext *types.APIRequest, obj runtime.Object, defaultNamespace string) types.APIObject {
	if defaultNamespace == "" {
		defaultNamespace = "default"
	}

	result := types.APIObject{
		Object: obj,
	}

	m, err := meta.Accessor(obj)
	if err != nil {
		return result
	}

	schemaID := a.schemaFactory.ByGVK(obj.GetObjectKind().GroupVersionKind())
	apiSchema := apiContext.Schemas.LookupSchema(schemaID)
	if apiSchema != nil {
		id := m.GetName()
		ns := m.GetNamespace()

		if ns == "" && attributes.Namespaced(apiSchema) {
			ns = defaultNamespace
		}

		if ns != "" {
			id = fmt.Sprintf("%s/%s", ns, id)
		}
		result.ID = id
		result.Type = apiSchema.ID

		if apiSchema.Store != nil {
			apiContext := apiContext.Clone()
			apiContext.Namespace = ns
			if obj, err := apiSchema.Store.ByID(apiContext.Clone(), apiSchema, m.GetName()); err == nil {
				return obj
			}
		}
	}

	return result
}

func (a *Apply) createApply(apiContext *types.APIRequest) (apply.Apply, error) {
	client, err := a.cg.K8sInterface(apiContext)
	if err != nil {
		return nil, err
	}

	apply := apply.New(client.Discovery(), func(gvr schema.GroupVersionResource) (dynamic.NamespaceableResourceInterface, error) {
		dynamicClient, err := a.cg.DynamicClient(apiContext)
		if err != nil {
			return nil, err
		}
		return dynamicClient.Resource(gvr), nil
	})

	return apply.
		WithDynamicLookup().
		WithContext(apiContext.Context()).
		WithSetID(uuid.New()), nil
}
