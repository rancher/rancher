package node

import (
	"fmt"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Formatter for Node
func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	etcd := convert.ToBool(resource.Values[client.NodeFieldEtcd])
	cp := convert.ToBool(resource.Values[client.NodeFieldControlPlane])
	worker := convert.ToBool(resource.Values[client.NodeFieldWorker])
	if !etcd && !cp && !worker {
		resource.Values[client.NodeFieldWorker] = true
	}

	// add nodeConfig link
	if err := apiContext.AccessControl.CanDo(v3.NodeDriverGroupVersionKind.Group, v3.NodeDriverResource.Name, "update", apiContext, resource.Values, apiContext.Schema); err == nil {
		resource.Links["nodeConfig"] = apiContext.URLBuilder.Link("nodeConfig", resource)
	}

	// remove link
	nodeTemplateID := resource.Values["nodeTemplateId"]
	customConfig := resource.Values["customConfig"]
	if nodeTemplateID == nil {
		delete(resource.Links, "nodeConfig")
	}

	if nodeTemplateID == nil && customConfig == nil {
		delete(resource.Links, "remove")
	}

	if convert.ToBool(resource.Values["unschedulable"]) {
		resource.AddAction(apiContext, "uncordon")
	} else {
		resource.AddAction(apiContext, "cordon")
	}
}

type ActionWrapper struct{}

func (a ActionWrapper) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case "cordon":
		return cordonUncordonNode(actionName, apiContext, true)

	case "uncordon":
		return cordonUncordonNode(actionName, apiContext, false)
	}

	return nil
}

func cordonUncordonNode(actionName string, apiContext *types.APIContext, cordon bool) error {
	var node map[string]interface{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &node); err != nil {
		return httperror.NewAPIError(httperror.InvalidReference, "Error accessing node")
	}
	schema := apiContext.Schemas.Schema(&managementschema.Version, client.NodeType)
	unschedulable := convert.ToBool(values.GetValueN(node, "unschedulable"))
	if cordon == unschedulable {
		return httperror.NewAPIError(httperror.InvalidAction, fmt.Sprintf("Node %s already %sed", apiContext.ID, actionName))
	}
	values.PutValue(node, convert.ToString(!unschedulable), "desiredNodeUnschedulable")
	if _, err := schema.Store.Update(apiContext, schema, node, apiContext.ID); err != nil && apierrors.IsNotFound(err) {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error updating node %s by %s : %s", apiContext.ID, actionName, err.Error()))
	}
	return nil
}
