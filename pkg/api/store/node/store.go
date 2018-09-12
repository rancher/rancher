package node

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/api/store/workload"
	"k8s.io/apimachinery/pkg/util/validation"
)

type nodeStore struct {
	types.Store
}

func SetupStore(schema *types.Schema) {
	schema.Store = &transform.Store{
		Store: nodeStore{
			Store: schema.Store,
		},
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			workload.SetPublicEnpointsFields(data)
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
	format(data)
	return n.Store.Update(apiContext, schema, data, id)
}

func format(data map[string]interface{}) {
	data["desiredNodeLabels"] = data["labels"]
	data["desiredNodeAnnotations"] = data["annotations"]
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
