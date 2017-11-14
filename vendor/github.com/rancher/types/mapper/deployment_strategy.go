package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

type DeploymentStrategyMapper struct {
}

func (d DeploymentStrategyMapper) FromInternal(data map[string]interface{}) {
	if values.GetValueN(data, "strategy", "type") != "Recreate" {
		values.PutValue(data, "Parallel", "deploymentStrategy", "kind")
		maxUnavailable := values.GetValueN(data, "strategy", "rollingUpdate", "maxUnavailable")
		maxSurge := values.GetValueN(data, "strategy", "rollingUpdate", "maxSurge")
		if !convert.IsEmpty(maxSurge) {
			values.PutValue(data, true, "deploymentStrategy", "parallelConfig", "startFirst")
			values.PutValue(data, convert.ToString(maxSurge), "batchSize")
		} else if !convert.IsEmpty(maxUnavailable) {
			values.PutValue(data, convert.ToString(maxUnavailable), "batchSize")
		}
	}
}

func (d DeploymentStrategyMapper) ToInternal(data map[string]interface{}) {
	batchSize := values.GetValueN(data, "batchSize")
	if convert.IsEmpty(batchSize) {
		batchSize = 1
	}

	batchSize, _ = convert.ToNumber(batchSize)

	kind, _ := values.GetValueN(data, "deploymentStrategy", "kind").(string)
	if kind == "" || kind == "Parallel" {
		startFirst, _ := values.GetValueN(data, "deploymentStrategy", "startFirst").(bool)
		if startFirst {
			values.PutValue(data, batchSize, "strategy", "rollingUpdate", "maxSurge")
		} else {
			values.PutValue(data, batchSize, "strategy", "rollingUpdate", "maxUnavailable")
		}
	}
}

func (d DeploymentStrategyMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
