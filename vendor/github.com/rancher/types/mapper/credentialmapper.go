package mapper

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

type CredentialMapper struct {
}

func (s CredentialMapper) FromInternal(data map[string]interface{}) {
	formatData(data)
	name := convert.ToString(values.GetValueN(data, "annotations", "field.cattle.io/name"))
	if name == "" {
		id := convert.ToString(values.GetValueN(data, "id"))
		if id != "" {
			values.PutValue(data, id, "annotations", "field.cattle.io/name")
		}
	}
	delete(data, "data")
}

func (s CredentialMapper) ToInternal(data map[string]interface{}) error {
	updateData(data)
	return nil
}

func (s CredentialMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}

func updateData(data map[string]interface{}) {
	stringData := map[string]string{}
	for key, val := range data {
		if val == nil {
			continue
		}
		if strings.HasSuffix(key, "Config") {
			for key2, val2 := range convert.ToMapInterface(val) {
				stringData[fmt.Sprintf("%s-%s", key, key2)] = convert.ToString(val2)
			}
			values.PutValue(data, stringData, "stringData")
			delete(data, key)
			return
		}
	}
}

func formatData(data map[string]interface{}) {
	secretData := convert.ToMapInterface(data["data"])
	getKey := func(data map[string]interface{}) string {
		for key := range data {
			splitKeys := strings.Split(key, "-")
			if len(splitKeys) != 2 {
				continue
			}
			if strings.HasSuffix(splitKeys[0], "Config") {
				return splitKeys[0]
			}
		}
		return ""
	}
	config := getKey(secretData)
	if config == "" {
		return
	}
	for key, val := range secretData {
		splitKeys := strings.Split(key, "-")
		if len(splitKeys) != 2 {
			continue
		}
		values.PutValue(data, convert.ToString(val), config, splitKeys[1])
	}
}
