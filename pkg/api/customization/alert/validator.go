package alert

import (
	"fmt"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	v3client "github.com/rancher/types/client/management/v3"
)

const monitoringEnabled = "MonitoringEnabled"

func ClusterAlertRuleValidator(resquest *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var clusterID string
	if resquest.ID != "" {
		clusterID, _ = ref.Parse(resquest.ID)
	} else {
		if cid := data["clusterId"]; cid != nil {
			clusterID = cid.(string)
		} else {
			return fmt.Errorf("cluster id is empty")
		}
	}
	var spec v3.ClusterAlertRuleSpec
	if err := convert.ToObj(data, &spec); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%v", err))
	}

	if spec.MetricRule != nil {
		var cluster v3client.Cluster
		if err := access.ByID(resquest, resquest.Version, v3client.ClusterType, clusterID, &cluster); err != nil {
			return err
		}

		if cluster.Conditions != nil {
			for _, v := range cluster.Conditions {
				if v.Type == monitoringEnabled && v.Status == "True" {
					return nil
				}
			}
		}
		return fmt.Errorf("if you want to use metric alert, need to enable monitoring for cluster %s", clusterID)
	}

	return nil
}

func ProjectAlertRuleValidator(resquest *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var clusterID string
	if resquest.ID != "" {
		clusterID, _ = ref.Parse(resquest.ID)
	} else {
		projectID := data["projectId"].(string)
		clusterID, _ = ref.Parse(projectID)
	}

	var spec v3.ProjectAlertRuleSpec
	if err := convert.ToObj(data, &spec); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%v", err))
	}

	if spec.MetricRule != nil {
		cluster := &v3.Cluster{}
		fmt.Println(resquest.Version, v3client.ClusterType, clusterID)
		if err := access.ByID(resquest, resquest.Version, v3client.ClusterType, clusterID, cluster); err != nil {
			return err
		}
		if v3.ClusterConditionMonitoringEnabled.IsTrue(cluster) {
			return nil
		}
		return fmt.Errorf("if you want to use metric alert, need to enable monitoring for cluster %s", clusterID)
	}

	return nil
}
