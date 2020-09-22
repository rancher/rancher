package catalog

import (
	"net/http"

	"github.com/rancher/apiserver/pkg/types"
	catalogtypes "github.com/rancher/rancher/pkg/api/steve/catalog/types"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/helmop"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"k8s.io/apimachinery/pkg/runtime"
	schema2 "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type operation struct {
	ops *helmop.Operations
}

func newOperation(
	helmop *helmop.Operations) *operation {
	return &operation{
		ops: helmop,
	}
}

func (o *operation) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	apiRequest := types.GetAPIContext(req.Context())

	user, ok := request.UserFrom(req.Context())
	if !ok {
		apiRequest.WriteError(validation.Unauthorized)
		return
	}

	var (
		op  *catalog.Operation
		err error
	)

	ns, name := nsAndName(apiRequest)
	switch apiRequest.Action {
	case "install":
		op, err = o.ops.Install(apiRequest.Context(), user, ns, name, req.Body)
	case "upgrade":
		op, err = o.ops.Upgrade(apiRequest.Context(), user, ns, name, req.Body)
	case "uninstall":
		op, err = o.ops.Uninstall(apiRequest.Context(), user, ns, name, req.Body)
	}

	switch apiRequest.Link {
	case "logs":
		err = o.ops.Log(apiRequest.Response, apiRequest.Request,
			apiRequest.Namespace, apiRequest.Name)
	}

	if err != nil {
		apiRequest.WriteError(err)
		return
	}

	if op == nil {
		return
	}

	apiRequest.WriteResponse(http.StatusCreated, types.APIObject{
		Type: "chartActionOutput",
		Object: &catalogtypes.ChartActionOutput{
			OperationName:      op.Name,
			OperationNamespace: op.Namespace,
		},
	})
}

func (o *operation) OnAdd(gvr schema2.GroupVersionResource, key string, obj runtime.Object) error {
	return o.ops.Impersonator.PurgeOldRoles(gvr, key, obj)
}

func (o *operation) OnChange(gvr schema2.GroupVersionResource, key string, obj, oldObj runtime.Object) error {
	return o.ops.Impersonator.PurgeOldRoles(gvr, key, obj)
}
