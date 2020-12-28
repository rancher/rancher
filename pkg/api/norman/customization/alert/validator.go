package alert

import (
	"fmt"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v3client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/ref"
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
	var spec v32.ClusterAlertRuleSpec
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
	projectID := data["projectId"].(string)

	var spec v32.ProjectAlertRuleSpec
	if err := convert.ToObj(data, &spec); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%v", err))
	}

	if spec.MetricRule != nil {
		project := &v3client.Project{}
		if err := access.ByID(resquest, resquest.Version, v3client.ProjectType, projectID, project); err != nil {
			return fmt.Errorf("access project by id failed, %v", err)
		}
		if project.Conditions != nil {
			for _, v := range project.Conditions {
				if v.Type == monitoringEnabled && v.Status == "True" {
					return nil
				}
			}
		}
		return fmt.Errorf("if you want to use metric alert, need to enable monitoring for project %s", projectID)
	}

	return nil
}
