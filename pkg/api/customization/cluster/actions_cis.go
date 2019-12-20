package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/controllers/user/cis"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	NumberOfRetriesForClusterUpdate = 3
	RetryIntervalInMilliseconds     = 5
)

func (a ActionHandler) runCisScan(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var err error

	canUpdateClusterFn := func(apiContext *types.APIContext) bool {
		cluster := map[string]interface{}{
			"id": apiContext.ID,
		}
		return apiContext.AccessControl.CanDo(v3.ClusterGroupVersionKind.Group, v3.ClusterResource.Name, "update", apiContext, cluster, apiContext.Schema) == nil
	}

	canUpdateCluster := canUpdateClusterFn(apiContext)
	logrus.Debugf("user: %v, canUpdateCluster: %v", apiContext.Request.Header.Get("Impersonate-User"), canUpdateCluster)
	if !canUpdateCluster {
		return httperror.NewAPIError(httperror.PermissionDenied, "can not run security scan")
	}

	data, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return errors.Wrap(err, "reading request body error")
	}

	cisScanConfig := &v3.CisScanConfig{}
	if err = json.Unmarshal(data, cisScanConfig); err != nil {
		return errors.Wrap(err, "unmarshaling input error")
	}

	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		return httperror.WrapAPIError(err, httperror.NotFound,
			fmt.Sprintf("cluster with id %v doesn't exist", apiContext.ID))
	}

	if cluster.DeletionTimestamp != nil {
		return httperror.NewAPIError(httperror.InvalidType,
			fmt.Sprintf("cluster with id %v is being deleted", apiContext.ID))
	}
	if !v3.ClusterConditionReady.IsTrue(cluster) {
		return httperror.WrapAPIError(err, httperror.ClusterUnavailable,
			fmt.Sprintf("cluster not ready"))
	}
	if _, ok := cluster.Annotations[v3.RunCisScanAnnotation]; ok {
		return httperror.WrapAPIError(err, httperror.Conflict,
			fmt.Sprintf("CIS scan already running on cluster"))
	}

	newCisScan := cis.NewCisScan(cluster, cisScanConfig)
	cisScan, err := a.ClusterScanClient.Create(newCisScan)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError,
			fmt.Sprintf("failed to create cis scan object"))
	}

	updatedCluster := cluster.DeepCopy()
	updatedCluster.Annotations[v3.RunCisScanAnnotation] = cisScan.Name

	// Can't add either too many retries or longer interval as this an API handler
	for i := 0; i < NumberOfRetriesForClusterUpdate; i++ {
		_, err = a.ClusterClient.Update(updatedCluster)
		if err == nil {
			break
		}
		time.Sleep(RetryIntervalInMilliseconds * time.Millisecond)
		cluster, err = a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
		if err != nil {
			logrus.Errorf("error fetching cluster with id %v: %v", apiContext.ID, err)
			continue
		}
		updatedCluster = cluster.DeepCopy()
		updatedCluster.Annotations[v3.RunCisScanAnnotation] = cisScan.Name
	}
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "failed to update cluster annotation for cis scan")
	}

	cisScanJSON, err := json.Marshal(cisScan)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError,
			fmt.Sprintf("failed to marshal cis scan object"))
	}

	logrus.Infof("CIS scan requested for cluster: %v", cluster.Name)
	apiContext.Response.Header().Set("Content-Type", "application/json")
	http.ServeContent(apiContext.Response, apiContext.Request, "clusterScan", time.Now(), bytes.NewReader(cisScanJSON))
	return nil
}
