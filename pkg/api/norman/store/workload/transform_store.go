package workload

import (
	"strings"

	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/api/norman/store/pod"
	"github.com/sirupsen/logrus"
)

var (
	hideOwnerReferenceKinds = map[string]bool{"CronJob": true, "Deployment": true}
)

func NewTransformStore(store types.Store) types.Store {
	return &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			var hide bool
			if opt != nil && opt.Options["hidden"] == "true" {
				hide = false
			} else if opt != nil && opt.Options["ByID"] == "true" {
				hide = false
			} else {
				hide = hideByOwner(data)
			}
			typeName := definition.GetType(data)
			name, _ := data["name"].(string)
			if hide && data["ownerReferences"] != nil {
				pod.SaveOwner(apiContext, typeName, name, data)
				return nil, nil
			}
			id, _ := data["id"].(string)
			if typeName != "" && id != "" {
				data["id"] = strings.ToLower(typeName) + ":" + id
			}
			SetPublicEndpointsFields(data)
			nodeName := convert.ToString(values.GetValueN(data, "nodeId"))
			if nodeName != "" {
				state := getState(data)
				nodeID := state[getKey(nodeName)]
				delete(data, "nodeId")
				values.PutValue(data, nodeID, "scheduling", "node", "nodeId")
			}
			setTransitioning(apiContext, data)
			return data, nil
		},
	}
}

func SetPublicEndpointsFields(data map[string]interface{}) {
	if val, ok := data["publicEndpoints"]; ok {
		eps := convert.ToInterfaceSlice(val)
		for _, ep := range eps {
			epMap, err := convert.EncodeToMap(ep)
			if err != nil {
				logrus.Errorf("Failed to convert public endpoint: %v", err)
				continue
			}
			epMap["serviceId"] = epMap["serviceName"]
			epMap["nodeId"] = epMap["nodeName"]
			epMap["ingressId"] = epMap["ingressName"]
		}
	}
}

func setTransitioning(apiContext *types.APIContext, data map[string]interface{}) {
	if data["transitioning"] == "yes" || data["transitioning"] == "error" {
		return
	}
	workloadType := convert.ToString(values.GetValueN(data, "type"))
	switch workloadType {
	case "/v3/project/schemas/deployment":
		update(data, "deploymentStatus", "replicas", "readyReplicas")
	case "/v3/project/schemas/daemonSet":
		update(data, "daemonSetStatus", "desiredNumberScheduled", "numberReady")
	case "/v3/project/schemas/statefulSet":
		update(data, "statefulSetStatus", "replicas", "readyReplicas")
	}
}

func update(data map[string]interface{}, statusField string, desiredField string, currField string) {
	if desiredNum, err := convert.ToNumber(values.GetValueN(data, statusField, desiredField)); err == nil {
		if readyNum, err := convert.ToNumber(values.GetValueN(data, statusField, currField)); err == nil {
			if desiredNum != readyNum {
				data["state"] = "updating"
				data["transitioning"] = "yes"
				data["transitioningMessage"] = "upgrading workload"
			}
		}
	}
}

func hideByOwner(data map[string]interface{}) bool {
	if data["ownerReferences"] != nil {
		owners := convert.ToMapSlice(data["ownerReferences"])
		for _, owner := range owners {
			ownerKind := convert.ToString(owner["kind"])
			if _, ok := hideOwnerReferenceKinds[ownerKind]; ok {
				return true
			}
		}
	}
	return false
}
