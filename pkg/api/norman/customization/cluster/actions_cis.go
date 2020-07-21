package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/controllers/managementuser/cis"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	cisScanConfig := &v32.CisScanConfig{}
	if err = json.Unmarshal(data, cisScanConfig); err != nil {
		return errors.Wrap(err, "unmarshaling input error")
	}

	cluster, err := a.ClusterClient.Get(apiContext.ID, v1.GetOptions{})
	if err != nil {
		return httperror.WrapAPIError(err, httperror.NotFound,
			fmt.Sprintf("cluster with id %v doesn't exist", apiContext.ID))
	}

	if cluster.Spec.WindowsPreferedCluster {
		return httperror.WrapAPIError(err, httperror.InvalidAction,
			fmt.Sprintf("cannot run scan on a windows cluster"))
	}
	if cluster.DeletionTimestamp != nil {
		return httperror.NewAPIError(httperror.InvalidType,
			fmt.Sprintf("cluster with id %v is being deleted", apiContext.ID))
	}
	if !v32.ClusterConditionReady.IsTrue(cluster) {
		return httperror.WrapAPIError(err, httperror.ClusterUnavailable,
			fmt.Sprintf("cluster not ready"))
	}
	if cluster.Status.CurrentCisRunName != "" {
		return httperror.WrapAPIError(err, httperror.Conflict,
			fmt.Sprintf("CIS scan already running on cluster"))
	}

	if cisScanConfig.OverrideBenchmarkVersion != "" {
		_, err := a.CisBenchmarkVersionLister.Get(namespace.GlobalNamespace, cisScanConfig.OverrideBenchmarkVersion)
		if err != nil {
			if kerrors.IsNotFound(err) {
				return httperror.WrapAPIError(err, httperror.InvalidOption,
					fmt.Sprintf("invalid override benchmark version specified"))
			}
			logrus.Errorf("error fetching cis benchmark version %v: %v", cisScanConfig.OverrideBenchmarkVersion, err)
			return httperror.WrapAPIError(err, httperror.ServerError,
				fmt.Sprintf("error fetching cis benchmark version %v", cisScanConfig.OverrideBenchmarkVersion))
		}
	}
	_, _, err = cis.GetBenchmarkVersionToUse(cisScanConfig.OverrideBenchmarkVersion,
		cluster.Spec.RancherKubernetesEngineConfig.Version,
		a.CisConfigLister, a.CisConfigClient,
		a.CisBenchmarkVersionLister, a.CisBenchmarkVersionClient,
	)
	if err != nil {
		return httperror.NewAPIError(httperror.MethodNotAllowed, err.Error())
	}

	isManual := true
	cisScan, err := cis.LaunchScan(
		isManual,
		cisScanConfig,
		cluster,
		a.ClusterClient,
		a.ClusterScanClient,
	)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, err.Error())
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
