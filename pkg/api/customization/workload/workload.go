package workload

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	projectschema "github.com/rancher/types/apis/project.cattle.io/v3/schema"
	projectclient "github.com/rancher/types/client/project/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
)

const (
	workloadRevisions    = "revisions"
	DeprecatedRollbackTo = "deprecated.deployment.rollback.to"
)

type ActionWrapper struct {
	ClusterManager *clustermanager.Manager
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
		clusterName := a.ClusterManager.ClusterName(apiContext)
		if clusterName == "" {
			return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Cluster name empty %s", deployment.ID))
		}
		clusterContext, err := a.ClusterManager.UserContext(clusterName)
		if err != nil {
			return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error getting cluster context %s", deployment.ID))
		}
		return a.rollbackDeployment(apiContext, clusterContext, actionName, deployment, namespace, name)

	case "pause":
		if deployment.Paused {
			return httperror.NewAPIError(httperror.InvalidAction, fmt.Sprintf("Deployment %s already paused", deployment.ID))
		}
		return updatePause(apiContext, true, deployment, "pause")

	case "resume":
		if !deployment.Paused {
			return httperror.NewAPIError(httperror.InvalidAction, fmt.Sprintf("Pause deployment %s before resume", deployment.ID))
		}
		return updatePause(apiContext, false, deployment, "resume")
	}
	return nil
}

func fetchRevisionFor(apiContext *types.APIContext, rollbackInput *projectclient.DeploymentRollbackInput, namespace string, name string, currRevision string) string {
	rollbackTo := rollbackInput.ReplicaSetID
	if rollbackTo == "" {
		revisionNum, _ := convert.ToNumber(currRevision)
		return convert.ToString(revisionNum - 1)
	}
	data := getRevisions(apiContext, namespace, name, rollbackTo)
	if len(data) > 0 {
		return convert.ToString(values.GetValueN(data[0], "workloadAnnotations", "deployment.kubernetes.io/revision"))
	}
	return ""
}

func getRevisions(apiContext *types.APIContext, namespace string, name string, requestedID string) []map[string]interface{} {
	data, replicaSets := []map[string]interface{}{}, []map[string]interface{}{}
	options := map[string]string{"hidden": "true"}
	conditions := []*types.QueryCondition{
		types.NewConditionFromString("namespaceId", types.ModifierEQ, []string{namespace}...),
	}
	if requestedID != "" {
		// want a specific replicaSet
		conditions = append(conditions, types.NewConditionFromString("id", types.ModifierEQ, []string{requestedID}...))
	}

	if err := access.List(apiContext, &projectschema.Version, projectclient.ReplicaSetType, &types.QueryOptions{Options: options, Conditions: conditions}, &replicaSets); err == nil {
		for _, replicaSet := range replicaSets {
			ownerReferences := convert.ToMapSlice(replicaSet["ownerReferences"])
			for _, ownerReference := range ownerReferences {
				kind := convert.ToString(ownerReference["kind"])
				ownerName := convert.ToString(ownerReference["name"])
				if kind == "Deployment" && name == ownerName {
					data = append(data, replicaSet)
					continue
				}
			}
		}
	}
	return data
}

func updatePause(apiContext *types.APIContext, value bool, deployment projectclient.Workload, actionName string) error {
	data, err := convert.EncodeToMap(deployment)
	if err == nil {
		values.PutValue(data, value, "paused")
		err = update(apiContext, data, deployment.ID)
	}
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error updating workload %s by %s : %s", deployment.ID, actionName, err.Error()))
	}
	return nil
}

func update(apiContext *types.APIContext, data map[string]interface{}, ID string) error {
	workloadSchema := apiContext.Schemas.Schema(&schema.Version, "workload")
	_, err := workloadSchema.Store.Update(apiContext, workloadSchema, data, ID)
	return err
}

func (a ActionWrapper) rollbackDeployment(apiContext *types.APIContext, clusterContext *config.UserContext,
	actionName string, deployment projectclient.Workload, namespace string, name string) error {
	input, err := handler.ParseAndValidateActionBody(apiContext, apiContext.Schemas.Schema(&projectschema.Version,
		projectclient.DeploymentRollbackInputType))
	if err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse action body: %v", err))
	}
	rollbackInput := &projectclient.DeploymentRollbackInput{}
	if err := mapstructure.Decode(input, rollbackInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}
	currRevision := deployment.WorkloadAnnotations["deployment.kubernetes.io/revision"]
	if currRevision == "1" {
		httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("No revision for rolling back %s", deployment.ID))
	}
	revision := fetchRevisionFor(apiContext, rollbackInput, namespace, name, currRevision)
	logrus.Debugf("rollbackInput %v", revision)
	if revision == "" {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("ReplicaSet %s doesn't exist for deployment %s", rollbackInput.ReplicaSetID, deployment.ID))
	}
	revisionNum, err := convert.ToNumber(revision)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error getting revision number %s for %s : %s", revision, deployment.ID, err.Error()))
	}
	data := map[string]interface{}{}
	data["kind"] = "DeploymentRollback"
	data["apiVersion"] = "extensions/v1beta1"
	data["name"] = name
	data["rollbackTo"] = map[string]interface{}{"revision": revisionNum}
	deploymentRollback, err := json.Marshal(data)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error getting DeploymentRollback for %s %s", rollbackInput.ReplicaSetID, err.Error()))
	}
	err = clusterContext.UnversionedClient.Post().Prefix("apis/extensions/v1beta1/").Namespace(namespace).
		Resource("deployments").Name(name).SubResource("rollback").Body(deploymentRollback).Do().Error()
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error updating workload %s by %s : %s", deployment.ID, actionName, err.Error()))
	}
	return nil
}

func (h Handler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	if apiContext.Link == workloadRevisions {
		var deployment projectclient.Workload
		if err := access.ByID(apiContext, &projectschema.Version, "workload", apiContext.ID, &deployment); err == nil {
			namespace, deploymentName := splitID(deployment.ID)
			data := getRevisions(apiContext, namespace, deploymentName, "")
			apiContext.Type = projectclient.ReplicaSetType
			apiContext.WriteResponse(http.StatusOK, data)
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
	resource.Actions["resume"] = apiContext.URLBuilder.ActionLinkByID(workloadSchema, workloadID, "resume")
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
