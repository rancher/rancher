package workload

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	projectclient "github.com/rancher/types/client/project/v3"
	"k8s.io/client-go/rest"
)

const (
	workloadRevisions    = "revisions"
	DeprecatedRollbackTo = "deprecated.deployment.rollback.to"
)

type ActionWrapper struct {
	Client rest.Interface
}

func (a ActionWrapper) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var deployment projectclient.Workload
	accessError := access.ByID(apiContext, &projectschema.Version, "workload", apiContext.ID, &deployment)
	if accessError != nil {
		return httperror.NewAPIError(httperror.InvalidReference, "Error accessing workload")
	}
	namespace, name := splitID(deployment.ID)
	switch actionName {
	case "rollback":
		actionInput, err := parse.ReadBody(apiContext.Request)
		if err != nil {
			return err
		}
		revision, _ := convert.ToNumber(actionInput["revision"])
		data := map[string]interface{}{}
		data["kind"] = "DeploymentRollback"
		data["apiVersion"] = "extensions/v1beta1"
		data["name"] = name
		data["rollbackTo"] = map[string]interface{}{"revision": revision}
		deploymentRollback, _ := json.Marshal(data)

		err = a.Client.Post().Prefix("apis/extensions/v1beta1/").Namespace(namespace).Resource("deployments").Name(name).SubResource("rollback").Body(deploymentRollback).Do().Error()
		if err != nil {
			return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error updating workload %s by %s", deployment.ID, actionName))
		}

	case "pause":
		data, err := convert.EncodeToMap(deployment)
		if err == nil {
			values.PutValue(data, !deployment.DeploymentConfig.Paused, "deploymentConfig", "paused")
			err = update(apiContext, data, deployment.ID)
		}
		if err != nil {
			return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error updating workload %s by %s", deployment.ID, actionName))
		}
	}
	return nil
}

func update(apiContext *types.APIContext, data map[string]interface{}, ID string) error {
	workloadSchema := apiContext.Schemas.Schema(&schema.Version, "workload")
	_, err := workloadSchema.Store.Update(apiContext, workloadSchema, data, ID)
	return err
}

func (h Handler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	if apiContext.Link == workloadRevisions {
		var deployment projectclient.Workload
		if err := access.ByID(apiContext, &projectschema.Version, "workload", apiContext.ID, &deployment); err == nil {
			namespace, deploymentName := splitID(deployment.ID)
			data, replicaSets := []map[string]interface{}{}, []map[string]interface{}{}
			options := map[string]string{"hidden": "true"}
			conditions := []*types.QueryCondition{
				types.NewConditionFromString("namespaceId", types.ModifierEQ, []string{namespace}...),
			}

			if err := access.List(apiContext, &projectschema.Version, projectclient.ReplicaSetType, &types.QueryOptions{Options: options, Conditions: conditions}, &replicaSets); err == nil {
				for _, replicaSet := range replicaSets {
					ownerReferences := convert.ToMapSlice(replicaSet["ownerReferences"])
					for _, ownerReference := range ownerReferences {
						kind := convert.ToString(ownerReference["kind"])
						name := convert.ToString(ownerReference["name"])
						if kind == "Deployment" && name == deploymentName {
							data = append(data, replicaSet)
						}
					}
				}
				apiContext.Type = projectclient.ReplicaSetType
				apiContext.WriteResponse(http.StatusOK, data)
			}
		}
		return nil
	}
	return httperror.NewAPIError(httperror.NotFound, "Link not found")
}

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	workloadID := resource.ID
	workloadSchema := apiContext.Schemas.Schema(&schema.Version, "workload")
	resource.Links["self"] = apiContext.URLBuilder.ResourceLinkByID(workloadSchema, workloadID)
	resource.Links["remove"] = apiContext.URLBuilder.ResourceLinkByID(workloadSchema, workloadID)
	resource.Links["update"] = apiContext.URLBuilder.ResourceLinkByID(workloadSchema, workloadID)
}

func DeploymentFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	workloadID := resource.ID
	workloadSchema := apiContext.Schemas.Schema(&schema.Version, "workload")
	Formatter(apiContext, resource)
	resource.Links["revisions"] = apiContext.URLBuilder.ResourceLinkByID(workloadSchema, workloadID) + "/" + workloadRevisions
	resource.Actions["pause"] = apiContext.URLBuilder.ActionLinkByID(workloadSchema, workloadID, "pause")
	resource.Actions["upgrade"] = apiContext.URLBuilder.ActionLinkByID(workloadSchema, workloadID, "upgrade")
	resource.Actions["rollback"] = apiContext.URLBuilder.ActionLinkByID(workloadSchema, workloadID, "rollback")
}

type Handler struct {
}

func splitID(id string) (string, string) {
	namespace := ""
	parts := strings.SplitN(id, ":", 3)
	if len(parts) == 3 {
		namespace = parts[1]
		id = parts[2]
	}

	return namespace, id
}
