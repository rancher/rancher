package workload

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	projectclient "github.com/rancher/rancher/pkg/client/generated/project/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	appsv1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	batchv1 "github.com/rancher/rancher/pkg/generated/norman/batch/v1"
	batchv1beta1 "github.com/rancher/rancher/pkg/generated/norman/batch/v1beta1"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/rbac"
	projectschema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	schema "github.com/rancher/rancher/pkg/schemas/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	k8sappsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	workloadRevisions    = "revisions"
	DeprecatedRollbackTo = "deprecated.deployment.rollback.to"
)

var (
	allowRedeployTypes     = map[string]bool{"cronJob": true, "deployment": true, "replicationController": true, "statefulSet": true, "daemonSet": true, "replicaSet": true}
	errInvalidWorkloadType = errors.New("invalid workload type")
)

type Config struct {
	ClusterManager *clustermanager.Manager
	Schemas        map[string]*types.Schema
}

func (a *Config) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var deployment projectclient.Workload
	accessError := access.ByID(apiContext, &projectschema.Version, "workload", apiContext.ID, &deployment)
	if accessError != nil {
		return httperror.NewAPIError(httperror.InvalidReference, "Error accessing workload")
	}
	workloadType, namespace, name := splitID(deployment.ID)

	// Create a RawResource with a minimal config. This will be used for the eventual "CanDo" method, which parses
	// the name and namespace from a RawResource object.
	rawResource := &types.RawResource{
		Values: map[string]interface{}{
			"id":          name,
			"namespaceId": namespace,
		},
	}
	if err := a.canUpdateWorkload(apiContext, rawResource, workloadType); err != nil {
		if err == errInvalidWorkloadType {
			return httperror.NewAPIError(httperror.InvalidType, errInvalidWorkloadType.Error())
		}
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}
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
	case "redeploy":
		return updateTimestamp(apiContext, deployment)
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
	var data, replicaSets []map[string]interface{}
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

func (a *Config) rollbackDeployment(apiContext *types.APIContext, clusterContext *config.UserContext,
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
	// if deployment's apiversion is apps/v1, we update the object, so getting it from etcd instead of cache
	depl, err := clusterContext.Apps.Deployments(namespace).Get(name, v1.GetOptions{})
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error getting deployment %v: %v", name, err))
	}
	deploymentVersion, err := k8sschema.ParseGroupVersion(depl.APIVersion)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error parsing api version for deployment %v: %v", name, err))
	}
	if deploymentVersion == k8sappsv1.SchemeGroupVersion {
		logrus.Debugf("Deployment apiversion is apps/v1")
		// DeploymentRollback & RollbackTo are deprecated in apps/v1
		// only way to rollback is update deployment podSpec with replicaSet podSpec
		split := strings.SplitN(rollbackInput.ReplicaSetID, ":", 3)
		if len(split) != 3 || split[0] != appsv1.ReplicaSetResource.SingularName {
			return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Invalid ReplicaSet %s", rollbackInput.ReplicaSetID))
		}
		replicaNamespace, replicaName := split[1], split[2]
		rs, err := clusterContext.Apps.ReplicaSets("").Controller().Lister().Get(replicaNamespace, replicaName)
		if err != nil {
			logrus.Debugf("ReplicaSet not found in cache, fetching from etcd")
			rs, err = clusterContext.Apps.ReplicaSets("").GetNamespaced(replicaNamespace, replicaName, v1.GetOptions{})
			if err != nil {
				return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("ReplicaSet %s not found for deployment %s", rollbackInput.ReplicaSetID, deployment.ID))
			}
		}
		toUpdateDepl := depl.DeepCopy()
		toUpdateDepl.Spec.Template.Spec = rs.Spec.Template.Spec
		_, err = clusterContext.Apps.Deployments("").Update(toUpdateDepl)
		if err != nil {
			return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error updating workload under apps/v1 %s by %s : %s", deployment.ID, actionName, err.Error()))
		}
		return nil
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
		Resource("deployments").Name(name).SubResource("rollback").Body(deploymentRollback).Do(apiContext.Request.Context()).Error()
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error updating workload %s by %s : %s", deployment.ID, actionName, err.Error()))
	}
	return nil
}
func updateTimestamp(apiContext *types.APIContext, workload projectclient.Workload) error {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	data, err := convert.EncodeToMap(workload)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "Failed to parse workload")
	}
	values.PutValue(data, timestamp, "annotations", "cattle.io/timestamp")
	err = update(apiContext, data, workload.ID)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error redeploying workload %s : %s", workload.ID, err.Error()))
	}
	return nil
}
func (h Handler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	if apiContext.Link == workloadRevisions {
		var deployment projectclient.Workload
		if err := access.ByID(apiContext, &projectschema.Version, "workload", apiContext.ID, &deployment); err == nil {
			_, namespace, deploymentName := splitID(deployment.ID)
			data := getRevisions(apiContext, namespace, deploymentName, "")
			apiContext.Type = projectclient.ReplicaSetType
			apiContext.WriteResponse(http.StatusOK, data)
		}
		return nil
	}
	return httperror.NewAPIError(httperror.NotFound, "Link not found")
}

func (a *Config) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	workloadID := resource.ID
	workloadSchema := apiContext.Schemas.Schema(&schema.Version, "workload")
	resource.Links["self"] = apiContext.URLBuilder.ResourceLinkByID(workloadSchema, workloadID)
	resource.Links["remove"] = apiContext.URLBuilder.ResourceLinkByID(workloadSchema, workloadID)
	resource.Links["update"] = apiContext.URLBuilder.ResourceLinkByID(workloadSchema, workloadID)
	// Add redeploy action to the workload types that support redeploy.
	_, ok := allowRedeployTypes[resource.Type]
	workloadType, _, _ := splitID(resource.ID)
	if ok && a.canUpdateWorkload(apiContext, resource, workloadType) == nil {
		resource.Actions["redeploy"] = apiContext.URLBuilder.ActionLinkByID(workloadSchema, workloadID, "redeploy")
	}

	delete(resource.Values, "nodeId")
}

func (a *Config) DeploymentFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	workloadID := resource.ID
	workloadSchema := apiContext.Schemas.Schema(&schema.Version, "workload")
	a.Formatter(apiContext, resource)
	resource.Links["revisions"] = apiContext.URLBuilder.ResourceLinkByID(workloadSchema, workloadID) + "/" + workloadRevisions
	workloadType, _, _ := splitID(resource.ID)
	if a.canUpdateWorkload(apiContext, resource, workloadType) == nil {
		resource.Actions["pause"] = apiContext.URLBuilder.ActionLinkByID(workloadSchema, workloadID, "pause")
		resource.Actions["resume"] = apiContext.URLBuilder.ActionLinkByID(workloadSchema, workloadID, "resume")
		resource.Actions["rollback"] = apiContext.URLBuilder.ActionLinkByID(workloadSchema, workloadID, "rollback")
	}
}

type Handler struct {
}

func splitID(id string) (string, string, string) {
	workloadType := ""
	namespace := ""
	parts := strings.SplitN(id, ":", 3)
	if len(parts) == 3 {
		workloadType = parts[0]
		namespace = parts[1]
		id = parts[2]
	}
	return workloadType, namespace, id
}

func (a *Config) canUpdateWorkload(apiContext *types.APIContext, resource *types.RawResource, workloadType string) error {
	var apiGroup string
	var pluralName string

	switch workloadType {
	case appsv1.DeploymentResource.SingularName:
		apiGroup = appsv1.GroupName
		pluralName = appsv1.DeploymentResource.Name
	case corev1.ReplicationControllerResource.SingularName:
		apiGroup = corev1.GroupName
		pluralName = corev1.ReplicationControllerResource.Name
	case appsv1.ReplicaSetResource.SingularName:
		apiGroup = appsv1.GroupName
		pluralName = appsv1.ReplicaSetResource.Name
	case appsv1.DaemonSetResource.SingularName:
		apiGroup = appsv1.GroupName
		pluralName = appsv1.DaemonSetResource.Name
	case appsv1.StatefulSetResource.SingularName:
		apiGroup = appsv1.GroupName
		pluralName = appsv1.StatefulSetResource.Name
	case batchv1.JobResource.SingularName:
		apiGroup = batchv1.GroupName
		pluralName = batchv1.JobResource.Name
	case batchv1beta1.CronJobResource.SingularName:
		apiGroup = batchv1beta1.GroupName
		pluralName = batchv1beta1.CronJobResource.Name
	default:
		logrus.Debugf("Invalid workload type: %s", workloadType)
		return errInvalidWorkloadType
	}

	return apiContext.AccessControl.CanDo(apiGroup, pluralName, "update", apiContext,
		rbac.ObjFromContext(apiContext, resource), a.Schemas[workloadType])
}
