package node

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/api/store/workload"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	client "github.com/rancher/rancher/pkg/types/client/management/v3"
	"k8s.io/apimachinery/pkg/util/validation"
)

const apiUpdate = "management.cattle.io/apiUpdate"

type nodeStore struct {
	types.Store
}

func SetupStore(schema *types.Schema) {
	schema.Store = &transform.Store{
		Store: nodeStore{
			Store: schema.Store,
		},
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			workload.SetPublicEndpointsFields(data)
			setState(data)
			return data, nil
		},
	}
}

func (n nodeStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	setName := false
	for _, cond := range opt.Conditions {
		if cond.Field == "name" && cond.ToCondition().Modifier == types.ModifierEQ {
			setName = true
			break
		}
	}

	datas, err := n.Store.List(apiContext, schema, opt)
	if err != nil || !setName {
		return datas, err
	}

	for _, data := range datas {
		if !convert.IsAPIObjectEmpty(data["name"]) {
			continue
		}

		if !convert.IsAPIObjectEmpty(data["nodeName"]) {
			data["name"] = data["nodeName"]
			continue
		}

		if !convert.IsAPIObjectEmpty(data["requestedHostname"]) {
			data["name"] = data["requestedHostname"]
			continue
		}
	}

	return datas, err
}

func (n nodeStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if err := n.validateDesiredTaints(data); err != nil {
		return nil, err
	}
	format(data)
	nodePoolID := n.getNodePoolID(apiContext, schema, data, "")
	if nodePoolID != "" {
		if err := n.validateHostname(schema, data); err != nil {
			return nil, err
		}
	}
	return n.Store.Create(apiContext, schema, data)
}

func (n nodeStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if err := n.validateDesiredTaints(data); err != nil {
		return nil, err
	}

	node := client.Node{}
	err := access.ByID(apiContext, apiContext.Version, apiContext.Type, id, &node)
	if err != nil {
		return nil, err
	}

	changed := v3.MetadataUpdate{
		Labels:      diff(convert.ToMapInterface(data["labels"]), node.Labels),
		Annotations: diff(convert.ToMapInterface(data["annotations"]), node.Annotations),
	}

	data["metadataUpdate"] = changed
	format(data)

	return n.Store.Update(apiContext, schema, data, id)
}

func diff(desired map[string]interface{}, actual map[string]string) (result v3.MapDelta) {
	if len(desired) == 0 {
		return
	}

	result.Delete = map[string]bool{}
	result.Add = map[string]string{}

	for k, v := range desired {
		if actual[k] != v {
			result.Add[k] = convert.ToString(v)
		}
	}

	for k := range actual {
		if _, exists := desired[k]; !exists {
			result.Delete[k] = true
		}
	}

	return
}

func format(data map[string]interface{}) {
	data["desiredNodeTaints"] = data["taints"]
	trueValue := true
	data["updateTaintsFromAPI"] = &trueValue
}

func setState(data map[string]interface{}) {
	if data["state"] == "draining" {
		return
	}
	if convert.ToBool(values.GetValueN(data, "unschedulable")) {
		conditions, _ := values.GetSlice(data, "conditions")
		for _, condition := range conditions {
			condType := values.GetValueN(condition, "type")
			if convert.ToString(condType) == "Drained" &&
				convert.ToString(values.GetValueN(condition, "status")) == "True" {
				data["state"] = "drained"
				return
			}
		}
		data["state"] = "cordoned"
	}
}

func (n nodeStore) getNodePoolID(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) string {
	_, ok := data["nodePoolId"]
	if ok {
		return data["nodePoolId"].(string)
	}
	if id != "" {
		existingNode, err := n.ByID(apiContext, schema, id)
		if err != nil {
			return ""
		}
		_, ok := existingNode["nodePoolId"].(string)
		if ok {
			return existingNode["nodePoolId"].(string)
		}
	}
	return ""
}

func (n nodeStore) validateHostname(schema *types.Schema, data map[string]interface{}) error {
	hostName := data["name"]
	if hostName != nil {
		errs := validation.IsDNS1123Label(hostName.(string))
		if len(errs) != 0 {
			return httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("invalid value %s: %s", hostName.(string),
				strings.Join(errs, ",")))
		}
	}

	return nil
}

func (n nodeStore) validateDesiredTaints(data map[string]interface{}) error {
	taints, ok := values.GetSlice(data, "taints")
	if !ok {
		return nil
	}
	uniqueSet := map[string]struct{}{}
	for _, taint := range taints {
		// key and effect is required field in API, so we can safely assume that key and effect exist.
		key, _ := values.GetValue(taint, "key")
		effect, _ := values.GetValue(taint, "effect")
		uniqueKey := fmt.Sprintf("%v:%v", key, effect)
		if _, ok := uniqueSet[uniqueKey]; ok {
			return httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("invalid taints, duplicated key %s and effect %s", key, effect))
		}
		uniqueSet[uniqueKey] = struct{}{}
	}
	return nil
}
