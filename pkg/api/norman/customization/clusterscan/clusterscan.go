package clusterscan

import (
	"net/http"
	"strconv"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/clustermanager"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/security-scan/pkg/kb-summarizer/report"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	errorState  = "error"
	failedState = "fail"
	passedState = "pass"
)

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	if err := apiContext.AccessControl.CanDo(mgmtv3.ClusterScanGroupVersionKind.Group, mgmtv3.ClusterScanResource.Name, "read", apiContext, resource.Values, apiContext.Schema); err == nil {
		s, ok := resource.Values["status"]
		if !ok {
			return
		}
		status := convert.ToMapInterface(s)
		failed := false
		completed := false
		runCompleted := false
		for _, cond := range convert.ToMapSlice(status["conditions"]) {
			if cond["type"] == string(v32.ClusterScanConditionCompleted) && cond["status"] == "True" {
				completed = true
			}
			if cond["type"] == string(v32.ClusterScanConditionFailed) && cond["status"] == "True" {
				failed = true
			}
			if cond["type"] == string(v32.ClusterScanConditionRunCompleted) && cond["status"] == "True" {
				runCompleted = true
			}
		}
		if runCompleted {
			resource.Values["state"] = "reporting"
			resource.Values["transitioning"] = "yes"
		}
		if failed {
			resource.Values["state"] = errorState
			return
		}
		if completed {
			resource.Links["report"] = apiContext.URLBuilder.Link("report", resource)
			cisScanStatusInterface, ok := status["cisScanStatus"]
			if !ok {
				resource.Values["state"] = errorState
				return
			}
			cisScanStatus := convert.ToMapInterface(cisScanStatusInterface)
			failInterface := cisScanStatus["fail"]
			fail, err := convert.ToNumber(failInterface)
			if err != nil {
				logrus.Errorf("error converting failInterface to int: %v", err)
				resource.Values["state"] = errorState
				return
			}
			if fail > 0 {
				resource.Values["state"] = failedState
			} else {
				resource.Values["state"] = passedState
			}
		}
	}
}

type Handler struct {
	CoreClient     corev1.Interface
	ClusterManager *clustermanager.Manager
}

func (h Handler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	var cs map[string]interface{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &cs); err != nil {
		return err
	}

	clusterID, clusterScanID := ref.Parse(cs["id"].(string))

	clusterContext, err := h.ClusterManager.UserContext(clusterID)
	if err != nil {
		return err
	}

	cm, err := clusterContext.Core.ConfigMaps(v32.DefaultNamespaceForCis).Get(clusterScanID, metav1.GetOptions{})
	if err != nil {
		return err
	}

	reportJSON, err := report.GetJSONBytes([]byte(cm.Data[v32.DefaultScanOutputFileName]))
	if err != nil {
		return err
	}

	apiContext.Response.Header().Set("Content-Length", strconv.Itoa(len(reportJSON)))
	apiContext.Response.Header().Set("Content-Type", "application/json")
	apiContext.Response.WriteHeader(http.StatusOK)
	_, err = apiContext.Response.Write(reportJSON)
	if err != nil {
		return err
	}

	return nil
}
