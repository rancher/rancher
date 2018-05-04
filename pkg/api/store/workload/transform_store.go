package workload

import (
	"strings"

	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/api/store/pod"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/sirupsen/logrus"
)

func NewTransformStore(store types.Store) types.Store {
	return &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			hide := true
			if opt != nil && opt.Options["hidden"] == "true" {
				hide = false
			}
			if opt != nil && opt.Options["ByID"] == "true" {
				hide = false
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
			SetPublicEnpointsFields(data)
			nodeName := convert.ToString(values.GetValueN(data, "nodeId"))
			if nodeName != "" {
				state := getState(data)
				_, nodeName := ref.Parse(state[getKey(nodeName)])
				data["nodeId"] = nodeName
			}
			return data, nil
		},
	}
}

func SetPublicEnpointsFields(data map[string]interface{}) {
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
