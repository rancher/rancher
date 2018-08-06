package mapper

import (
	"sort"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"k8s.io/api/core/v1"
)

type EnvironmentMapper struct {
}

func (e EnvironmentMapper) FromInternal(data map[string]interface{}) {
	var env []v1.EnvVar
	var envFrom []v1.EnvFromSource

	envMap := map[string]interface{}{}
	var envFromMaps []interface{}

	if err := convert.ToObj(data["env"], &env); err == nil {
		for _, envVar := range env {
			if envVar.ValueFrom == nil {
				envMap[envVar.Name] = envVar.Value
				continue
			}

			if envVar.ValueFrom.FieldRef != nil {
				envFromMaps = append(envFromMaps, map[string]interface{}{
					"source":     "field",
					"sourceName": envVar.ValueFrom.FieldRef.FieldPath,
					"targetKey":  envVar.Name,
				})
			}
			if envVar.ValueFrom.ResourceFieldRef != nil {
				envFromMaps = append(envFromMaps, map[string]interface{}{
					"source":     "resource",
					"sourceName": envVar.ValueFrom.ResourceFieldRef.ContainerName,
					"sourceKey":  envVar.ValueFrom.ResourceFieldRef.Resource,
					"divisor":    envVar.ValueFrom.ResourceFieldRef.Divisor,
					"targetKey":  envVar.Name,
				})
			}
			if envVar.ValueFrom.ConfigMapKeyRef != nil {
				envFromMaps = append(envFromMaps, map[string]interface{}{
					"source":     "configMap",
					"sourceName": envVar.ValueFrom.ConfigMapKeyRef.Name,
					"sourceKey":  envVar.ValueFrom.ConfigMapKeyRef.Key,
					"optional":   getValue(envVar.ValueFrom.ConfigMapKeyRef.Optional),
					"targetKey":  envVar.Name,
				})
			}
			if envVar.ValueFrom.SecretKeyRef != nil {
				envFromMaps = append(envFromMaps, map[string]interface{}{
					"source":     "secret",
					"sourceName": envVar.ValueFrom.SecretKeyRef.Name,
					"sourceKey":  envVar.ValueFrom.SecretKeyRef.Key,
					"optional":   getValue(envVar.ValueFrom.SecretKeyRef.Optional),
					"targetKey":  envVar.Name,
				})
			}
		}
	}

	if err := convert.ToObj(data["envFrom"], &envFrom); err == nil {
		for _, envVar := range envFrom {
			if envVar.SecretRef != nil {
				envFromMaps = append(envFromMaps, map[string]interface{}{
					"source":     "secret",
					"sourceName": envVar.SecretRef.Name,
					"prefix":     envVar.Prefix,
					"optional":   getValue(envVar.SecretRef.Optional),
					"type":       "/v3/project/schemas/environmentFrom",
				})
			}
			if envVar.ConfigMapRef != nil {
				envFromMaps = append(envFromMaps, map[string]interface{}{
					"source":     "configMap",
					"sourceName": envVar.ConfigMapRef.Name,
					"prefix":     envVar.Prefix,
					"optional":   getValue(envVar.ConfigMapRef.Optional),
					"type":       "/v3/project/schemas/environmentFrom",
				})
			}
		}
	}

	delete(data, "env")
	delete(data, "envFrom")

	if len(envMap) > 0 {
		data["environment"] = envMap
	}
	if len(envFromMaps) > 0 {
		data["environmentFrom"] = envFromMaps
	}
}

func (e EnvironmentMapper) ToInternal(data map[string]interface{}) error {
	var envVar []map[string]interface{}
	var envVarFrom []map[string]interface{}

	var orderedKeys []string
	environment := convert.ToMapInterface(data["environment"])
	for k := range environment {
		orderedKeys = append(orderedKeys, k)
	}
	sort.Strings(orderedKeys)
	for _, key := range orderedKeys {
		envVar = append(envVar, map[string]interface{}{
			"name":  key,
			"value": environment[key],
		})
	}

	for _, value := range convert.ToMapSlice(data["environmentFrom"]) {
		source := convert.ToString(value["source"])
		if source == "" {
			continue
		}

		targetKey := convert.ToString(value["targetKey"])
		sourceKey := convert.ToString(value["sourceKey"])
		if targetKey == "" && sourceKey == "" {
			switch source {
			case "secret":
				envVarFrom = append(envVarFrom, map[string]interface{}{
					"prefix": value["prefix"],
					"secretRef": map[string]interface{}{
						"name":     value["sourceName"],
						"optional": value["optional"],
					},
				})
			case "configMap":
				envVarFrom = append(envVarFrom, map[string]interface{}{
					"prefix": value["prefix"],
					"configMapRef": map[string]interface{}{
						"name":     value["sourceName"],
						"optional": value["optional"],
					},
				})
			}
		} else {
			if targetKey == "" {
				targetKey = sourceKey
			}
			switch source {
			case "field":
				envVar = append(envVar, map[string]interface{}{
					"name": targetKey,
					"valueFrom": map[string]interface{}{
						"fieldRef": map[string]interface{}{
							"fieldPath": value["sourceName"],
						},
					},
				})
			case "resource":
				envVar = append(envVar, map[string]interface{}{
					"name": targetKey,
					"valueFrom": map[string]interface{}{
						"resourceFieldRef": map[string]interface{}{
							"containerName": value["sourceName"],
							"resource":      value["sourceKey"],
							"divisor":       value["divisor"],
						},
					},
				})
			case "configMap":
				envVar = append(envVar, map[string]interface{}{
					"name": targetKey,
					"valueFrom": map[string]interface{}{
						"configMapKeyRef": map[string]interface{}{
							"name":     value["sourceName"],
							"key":      value["sourceKey"],
							"optional": value["optional"],
						},
					},
				})
			case "secret":
				envVar = append(envVar, map[string]interface{}{
					"name": targetKey,
					"valueFrom": map[string]interface{}{
						"secretKeyRef": map[string]interface{}{
							"name":     value["sourceName"],
							"key":      value["sourceKey"],
							"optional": value["optional"],
						},
					},
				})
			}
		}
	}

	delete(data, "environment")
	delete(data, "environmentFrom")
	data["env"] = envVar
	data["envFrom"] = envVarFrom

	return nil
}

func (e EnvironmentMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	delete(schema.ResourceFields, "env")
	delete(schema.ResourceFields, "envFrom")
	return nil
}

func getValue(optional *bool) bool {
	if optional != nil {
		return *optional
	}
	return false
}
