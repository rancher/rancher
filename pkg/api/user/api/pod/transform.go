package pod

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/rancher/pkg/api/user/api/workload"
)

func Transform(context *types.APIContext, data map[string]interface{}) (map[string]interface{}, error) {
	if data == nil {
		return data, nil
	}
	mapping, err := workload.OwnerMap(context)
	if err != nil {
		return nil, err
	}

	return assignID(data, mapping), nil
}

func assignID(data map[string]interface{}, mapping map[string]string) map[string]interface{} {
	owner := workload.ResolveWorkloadID(data, mapping)
	if owner != "" {
		data["workloadId"] = owner
	}

	return data
}

func ListTransform(context *types.APIContext, data []map[string]interface{}) ([]map[string]interface{}, error) {
	mapping, err := workload.OwnerMap(context)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, item := range data {
		result = append(result, assignID(item, mapping))
	}

	return result, nil
}

func StreamTransform(context *types.APIContext, data chan map[string]interface{}) (chan map[string]interface{}, error) {
	mapping, err := workload.OwnerMap(context)
	if err != nil {
		return nil, err
	}

	result := make(chan map[string]interface{})
	go func() {
		for item := range data {
			typeName := definition.GetType(item)
			if strings.Contains(typeName, "replica") || strings.Contains(typeName, "deployment") {
				tempMapping, err := workload.OwnerMap(context)
				if err == nil {
					mapping = tempMapping
				}
			}

			result <- assignID(item, mapping)
		}
		close(result)
	}()

	return result, nil
}
