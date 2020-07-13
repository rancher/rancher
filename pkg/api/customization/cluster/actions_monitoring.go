package cluster

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/monitoring"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
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
	answers, version := monitoring.GetOverwroteAppAnswersAndVersion(cluster.Annotations)
	encodeAnswers, err := convert.EncodeToMap(answers)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to parse response")
	}
	resp := map[string]interface{}{
		"answers": encodeAnswers,
		"type":    "monitoringOutput",
	}
	if version != "" {
		resp["version"] = version
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

	data, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "unable to read request content")
	}
	var input v3.MonitoringInput
	if err = json.Unmarshal(data, &input); err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "failed to parse request content")
	}

	cluster = cluster.DeepCopy()
	cluster.Annotations = monitoring.AppendAppOverwritingAnswers(cluster.Annotations, string(data))

	_, err = a.ClusterClient.Update(cluster)
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

	data, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "unable to read request content")
	}
	var input v3.MonitoringInput
	if err = json.Unmarshal(data, &input); err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "failed to parse request content")
	}

	cluster = cluster.DeepCopy()
	cluster.Spec.EnableClusterMonitoring = true
	cluster.Annotations = monitoring.AppendAppOverwritingAnswers(cluster.Annotations, string(data))

	_, err = a.ClusterClient.Update(cluster)
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

	cluster = cluster.DeepCopy()
	cluster.Spec.EnableClusterMonitoring = false

	_, err = a.ClusterClient.Update(cluster)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to disable monitoring")
	}

	apiContext.WriteResponse(http.StatusNoContent, map[string]interface{}{})
	return nil
}
