package cluster

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/controllers/user/cis"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a ActionHandler) runCISScan(actionName string, action *types.Action, apiContext *types.APIContext) error {
	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		return httperror.WrapAPIError(err, httperror.NotFound,
			fmt.Sprintf("cluster with id %v doesn't exist", apiContext.ID))
	}
	if cluster.DeletionTimestamp != nil {
		return httperror.NewAPIError(httperror.InvalidType,
			fmt.Sprintf("cluster with id %v is being deleted", apiContext.ID))
	}

	if _, ok := cluster.Annotations[cis.RunCISScanAnnotation]; ok {
		return httperror.WrapAPIError(err, httperror.Conflict,
			fmt.Sprintf("CIS scan already running on cluster"))
	}

	// TODO: Check if there are other cluster states when we can't run the scan
	updatedCluster := cluster.DeepCopy()
	updatedCluster.Annotations[cis.RunCISScanAnnotation] = "true"

	_, err = a.ClusterClient.Update(updatedCluster)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to start CIS scan")
	}

	logrus.Infof("CIS scan requested")
	apiContext.WriteResponse(http.StatusOK, map[string]interface{}{})
	return nil
}
