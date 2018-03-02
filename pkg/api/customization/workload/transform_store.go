package workload

import (
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/norman/types/values"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	managementv3 "github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
)

func New(store types.Store) types.Store {
	return &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			hide := true
			if opt != nil && opt.Options["hidden"] == "true" {
				hide = false
			}
			if hide && data["ownerReferences"] != nil {
				return nil, nil
			}
			typeName := definition.GetType(data)
			id, _ := data["id"].(string)
			if typeName != "" && id != "" {
				data["id"] = strings.ToLower(typeName) + ":" + id
			}
			setNodeName(apiContext, data)
			nodeName := convert.ToString(values.GetValueN(data, "nodeId"))
			if nodeName != "" {
				state := getState(data)
				data["nodeId"] = state[getKey(nodeName)]
			}
			return data, nil
		},
	}
}

func setNodeName(apiContext *types.APIContext, data map[string]interface{}) {
	if val, ok := data["publicEndpoints"]; ok {
		var nodes []managementv3.Node
		nodeNameToID := map[string]string{}
		if err := access.List(apiContext, &managementschema.Version, managementv3.NodeType, &types.QueryOptions{}, &nodes); err == nil {
			for _, node := range nodes {
				nodeNameToID[node.NodeName] = node.ID
			}
		}
		eps := convert.ToInterfaceSlice(val)
		for _, ep := range eps {
			epMap, err := convert.EncodeToMap(ep)
			if err != nil {
				logrus.Errorf("Failed to convert public endpoint: %v", err)
				continue
			}
			epMap["serviceId"] = epMap["serviceName"]
			if convert.IsEmpty(epMap["nodeName"]) {
				continue
			}
			epMap["nodeId"] = nodeNameToID[convert.ToString(epMap["nodeName"])]
		}
	}
}
