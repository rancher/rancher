package cluster

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/monitoring"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a ActionHandler) viewMonitoring(actionName string, action *types.Action, apiContext *types.APIContext) error {
	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		return httperror.WrapAPIError(err, httperror.NotFound, "none existent Cluster")
	}
	if cluster.DeletionTimestamp != nil {
		return httperror.NewAPIError(httperror.InvalidType, "deleting Cluster")
	}

	if !cluster.Spec.EnableClusterMonitoring {
		return httperror.NewAPIError(httperror.InvalidState, "disabling Monitoring")
	}

	// need to support `map[string]string` as entry value type in norman Builder.convertMap
	monitoringInput := monitoring.GetMonitoringInput(cluster.Annotations)
	encodedAnswers, err := convert.EncodeToMap(monitoringInput.Answers)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to parse response")
	}
	encodedAnswersSetString, err := convert.EncodeToMap(monitoringInput.AnswersSetString)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to parse response")
	}
	resp := map[string]interface{}{
		"answers":          encodedAnswers,
		"answersSetString": encodedAnswersSetString,
		"type":             "monitoringOutput",
	}
	if monitoringInput.Version != "" {
		resp["version"] = monitoringInput.Version
	}

	apiContext.WriteResponse(http.StatusOK, resp)
	return nil
}

func (a ActionHandler) editMonitoring(actionName string, action *types.Action, apiContext *types.APIContext) error {
	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		return httperror.WrapAPIError(err, httperror.NotFound, "none existent Cluster")
	}
	if cluster.DeletionTimestamp != nil {
		return httperror.NewAPIError(httperror.InvalidType, "deleting Cluster")
	}

	if !cluster.Spec.EnableClusterMonitoring {
		return httperror.NewAPIError(httperror.InvalidState, "disabling Monitoring")
	}

	data, err := io.ReadAll(apiContext.Request.Body)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "unable to read request content")
	}
	var input v32.MonitoringInput
	if err = json.Unmarshal(data, &input); err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "failed to parse request content")
	}

	if err := a.validateChartCompatibility(input.Version, apiContext.ID); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}

	err = updateClusterWithRetryOnConflict(a.ClusterClient, cluster, func(cluster *v3.Cluster) *v3.Cluster {
		cluster.Annotations = monitoring.AppendAppOverwritingAnswers(cluster.Annotations, string(data))
		return cluster
	})
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to upgrade monitoring")
	}

	apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
	return nil
}

func (a ActionHandler) enableMonitoring(actionName string, action *types.Action, apiContext *types.APIContext) error {
	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		return httperror.WrapAPIError(err, httperror.NotFound, "none existent Cluster")
	}
	if cluster.DeletionTimestamp != nil {
		return httperror.NewAPIError(httperror.InvalidType, "deleting Cluster")
	}

	if cluster.Spec.EnableClusterMonitoring {
		apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
		return nil
	}

	data, err := io.ReadAll(apiContext.Request.Body)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "unable to read request content")
	}
	var input v32.MonitoringInput
	if err = json.Unmarshal(data, &input); err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "failed to parse request content")
	}

	if err := a.validateChartCompatibility(input.Version, apiContext.ID); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}

	err = updateClusterWithRetryOnConflict(a.ClusterClient, cluster, func(cluster *v3.Cluster) *v3.Cluster {
		cluster.Spec.EnableClusterMonitoring = true
		cluster.Annotations = monitoring.AppendAppOverwritingAnswers(cluster.Annotations, string(data))
		return cluster
	})
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to enable monitoring")
	}

	apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
	return nil
}

func (a ActionHandler) disableMonitoring(actionName string, action *types.Action, apiContext *types.APIContext) error {
	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		return httperror.WrapAPIError(err, httperror.NotFound, "none existent Cluster")
	}
	if cluster.DeletionTimestamp != nil {
		return httperror.NewAPIError(httperror.InvalidType, "deleting Cluster")
	}

	if !cluster.Spec.EnableClusterMonitoring {
		apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
		return nil
	}

	err = updateClusterWithRetryOnConflict(a.ClusterClient, cluster, func(cluster *v3.Cluster) *v3.Cluster {
		cluster.Spec.EnableClusterMonitoring = false
		return cluster
	})
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to disable monitoring")
	}

	apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
	return nil
}

func (a ActionHandler) validateChartCompatibility(version, clusterName string) error {
	if version == "" {
		return nil
	}
	templateVersionID := fmt.Sprintf("system-library-rancher-monitoring-%s", version)
	templateVersion, err := a.CatalogTemplateVersionLister.Get("cattle-global-data", templateVersionID)
	if err != nil {
		return err
	}
	return a.CatalogManager.ValidateChartCompatibility(templateVersion, clusterName, "")
}
