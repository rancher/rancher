package workload

import (
	"fmt"

	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/types/apis/project.cattle.io/v3/schema"
	"github.com/rancher/types/client/project/v3"
)

func OwnerMap(context *types.APIContext) (map[string]string, error) {
	result := map[string]string{}

	var workloads []client.Workload
	if err := access.List(context, &schema.Version, client.WorkloadType, &types.QueryOptions{
		Options: map[string]string{
			"hidden": "true",
		},
	}, &workloads); err != nil {
		return nil, err
	}

	for _, workload := range workloads {
		for _, owner := range workload.OwnerReferences {
			if owner.Controller != nil && *owner.Controller {
				id := key(definition.GetShortTypeFromFull(workload.Type), workload.Name)
				result[id] = key(owner.Kind, owner.Name)
			}
		}
	}

	return result, nil
}

func ResolveWorkloadID(data map[string]interface{}, owners map[string]string) string {
	workloadID := ""

	ownerReferences, ok := values.GetSlice(data, "ownerReferences")
	if !ok {
		return ""
	}

	for _, ownerReference := range ownerReferences {
		controller, _ := ownerReference["controller"].(bool)
		if !controller {
			continue
		}

		kind, _ := ownerReference["kind"].(string)
		name, _ := ownerReference["name"].(string)
		parent := key(kind, name)
		workloadID = ""
		for parent != "" {
			workloadID = parent
			parent = owners[workloadID]
		}
	}

	if workloadID == "" {
		return ""
	}

	parts := strings.SplitN(workloadID, "/", 2)
	namespace, _ := data["namespaceId"].(string)
	return fmt.Sprintf("%s:%s:%s", parts[0], namespace, parts[1])
}

func key(kind, name string) string {
	return strings.ToLower(fmt.Sprintf("%s/%s", kind, name))
}
