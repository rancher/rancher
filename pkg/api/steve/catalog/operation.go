package catalog

import (
	"net/http"

	"github.com/rancher/apiserver/pkg/types"
	catalogtypes "github.com/rancher/rancher/pkg/api/steve/catalog/types"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/helmop"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v2/pkg/schemas/validation"
	"k8s.io/apimachinery/pkg/runtime"
	schema2 "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// operation implements the http handler interface.
type operation struct {
	// ops is the Operation struct which contains functions
	// related to helm such as helm install, helm uninstall etc.
	ops *helmop.Operations
	// imageOveride is the location of the rancher shell image which is used
	// for running helm commands such as install, upgrade and uninstall.
	imageOverride string
}

// newOperation parses and returns an object operation updating the imageOverride field
// based on the clusterRegistry parameter. If clusterRegistry is not an empty string
// the imageOverride value will be prefixed with clusterRegistry value.
func newOperation(
	helmop *helmop.Operations, clusterRegistry string) *operation {
	var imageOverride string
	if clusterRegistry != "" {
		imageOverride = clusterRegistry + "/" + settings.ShellImage.Get()
	}
	return &operation{
		ops:           helmop,
		imageOverride: imageOverride,
	}
}

// ServeHTTP calls corresponding Operation functions based on the type of the api request.
// It uses the rancher apiserver package to parse the request and know the type of it.
// The types are documented in the rancher apiserver package. After parsing, it then
// checks if the request is authorised by checking the user field in the request.
//
// For example, if the api request is for installing a chart, then it will call the
// install function of the Operation struct.
//
// All chart actions (install, upgrade, and uninstall) are served through this method.
func (o *operation) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Get the APIContext from the current request's context. This APIContext
	// encapsulates the details of the API request, which will be used to
	// determine the necessary operation and respond accordingly.
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
		op, err = o.ops.Install(apiRequest.Context(), user, ns, name, req.Body, o.imageOverride)
	case "upgrade":
		op, err = o.ops.Upgrade(apiRequest.Context(), user, ns, name, req.Body, o.imageOverride)
	case "uninstall":
		op, err = o.ops.Uninstall(apiRequest.Context(), user, ns, name, req.Body, o.imageOverride)
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

// OnAdd is registered as a callback of a Kubernetes Informer.
// It is invoked when a new object is added to the Kubernetes cluster.
// It purges old roles related to the object being added.
// These old roles will be purged upon timeout.
func (o *operation) OnAdd(gvk schema2.GroupVersionKind, key string, obj runtime.Object) error {
	return o.ops.Impersonator.PurgeOldRoles(gvk, key, obj)
}

// OnChange is registered as a callback of a Kubernetes Informer.
// It is invoked when an existing object is modified inside the Kubernetes cluster.
// It purges old roles related to the object being modified.
// These old roles will be purged upon timeout.
func (o *operation) OnChange(gvk schema2.GroupVersionKind, key string, obj, oldObj runtime.Object) error {
	return o.ops.Impersonator.PurgeOldRoles(gvk, key, obj)
}
