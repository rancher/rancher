package clusters

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/pborman/uuid"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/stores/proxy"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/yaml"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type Apply struct {
	cg proxy.ClientGetter
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

	rw.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(rw).Encode(&ApplyOutput{
		Resources: objs,
	})
	if err != nil {
		apiContext.WriteError(err)
	}
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
