package catalog

import (
	"net/http"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/api/steve/catalog/helmop"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	"github.com/rancher/steve/pkg/stores/proxy"
	corev1controllers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"k8s.io/apimachinery/pkg/runtime"
	schema2 "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type operation struct {
	ops *helmop.Operations
}

func newOperation(
	cg proxy.ClientGetter,
	catalog catalogcontrollers.Interface,
	secrets corev1controllers.SecretClient) *operation {
	return &operation{
		ops: helmop.NewOperations(cg, catalog, secrets),
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

	switch apiRequest.Action {
	case "install":
		op, err = o.ops.Install(apiRequest.Context(), user, getKind(apiRequest),
			apiRequest.Namespace, apiRequest.Name, req.Body)
	case "upgrade":
		op, err = o.ops.Upgrade(apiRequest.Context(), user, getKind(apiRequest),
			apiRequest.Namespace, apiRequest.Name, req.Body)
	case "rollback":
		op, err = o.ops.Rollback(apiRequest.Context(), user,
			apiRequest.Namespace, apiRequest.Name, req.Body)
	case "uninstall":
		op, err = o.ops.Uninstall(apiRequest.Context(), user,
			apiRequest.Namespace, apiRequest.Name, req.Body)
	case "log":
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
		Object: &catalog.ChartActionOutput{
			OperationName:      op.Name,
			OperationNamespace: op.Namespace,
		},
	})
}

func getKind(apiRequest *types.APIRequest) string {
	if apiRequest.Type == "catalog.catalog.io.clusterrepo" {
		return "ClusterRepo"
	}
	return "Repo"
}

func (o *operation) OnAdd(gvr schema2.GroupVersionResource, key string, obj runtime.Object) error {
	return o.ops.Impersonator.PurgeOldRoles(gvr, key, obj)
}

func (o *operation) OnChange(gvr schema2.GroupVersionResource, key string, obj, oldObj runtime.Object) error {
	return o.ops.Impersonator.PurgeOldRoles(gvr, key, obj)
}
