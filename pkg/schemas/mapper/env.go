package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v1 "k8s.io/api/core/v1"
)

type EnvironmentMapper struct {
}

func (e EnvironmentMapper) FromInternal(data map[string]interface{}) {
	var env []v1.EnvVar
	var envFrom []v1.EnvFromSource
	var environment, environmentFrom []interface{}

	if err := convert.ToObj(data["env"], &env); err == nil {
		for _, envVar := range env {
			if envVar.ValueFrom == nil {
				environment = append(environment, map[string]interface{}{
					"name":  envVar.Name,
					"value": envVar.Value,
				})
				continue
			}
			if envVar.ValueFrom.FieldRef != nil {
				environment = append(environment, map[string]interface{}{
					"name": envVar.Name,
					"valueFrom": map[string]interface{}{
						"source":     "field",
						"sourceName": envVar.ValueFrom.FieldRef.FieldPath,
						"targetKey":  envVar.Name,
					},
				})
			}
			if envVar.ValueFrom.ResourceFieldRef != nil {
				environment = append(environment, map[string]interface{}{
					"name": envVar.Name,
					"valueFrom": map[string]interface{}{
						"source":     "resource",
						"sourceName": envVar.ValueFrom.ResourceFieldRef.ContainerName,
						"sourceKey":  envVar.ValueFrom.ResourceFieldRef.Resource,
						"divisor":    envVar.ValueFrom.ResourceFieldRef.Divisor,
						"targetKey":  envVar.Name,
					},
				})
			}
			if envVar.ValueFrom.ConfigMapKeyRef != nil {
				environment = append(environment, map[string]interface{}{
					"name": envVar.Name,
					"valueFrom": map[string]interface{}{
						"source":     "configMap",
						"sourceName": envVar.ValueFrom.ConfigMapKeyRef.Name,
						"sourceKey":  envVar.ValueFrom.ConfigMapKeyRef.Key,
						"optional":   getValue(envVar.ValueFrom.ConfigMapKeyRef.Optional),
						"targetKey":  envVar.Name,
					},
				})
			}
			if envVar.ValueFrom.SecretKeyRef != nil {
				environment = append(environment, map[string]interface{}{
					"name": envVar.Name,
					"valueFrom": map[string]interface{}{
						"source":     "secret",
						"sourceName": envVar.ValueFrom.SecretKeyRef.Name,
						"sourceKey":  envVar.ValueFrom.SecretKeyRef.Key,
						"optional":   getValue(envVar.ValueFrom.SecretKeyRef.Optional),
						"targetKey":  envVar.Name,
					},
				})
			}
		}
	}
	if err := convert.ToObj(data["envFrom"], &envFrom); err == nil {
		for _, envFromSource := range envFrom {
			if envFromSource.SecretRef != nil {
				environmentFrom = append(environmentFrom, map[string]interface{}{
					"source":     "secret",
					"sourceName": envFromSource.SecretRef.Name,
					"prefix":     envFromSource.Prefix,
					"optional":   getValue(envFromSource.SecretRef.Optional),
					"type":       "/v3/project/schemas/environmentFrom",
				})
			}
			if envFromSource.ConfigMapRef != nil {
				environmentFrom = append(environmentFrom, map[string]interface{}{
					"source":     "configMap",
					"sourceName": envFromSource.ConfigMapRef.Name,
					"prefix":     envFromSource.Prefix,
					"optional":   getValue(envFromSource.ConfigMapRef.Optional),
					"type":       "/v3/project/schemas/environmentFrom",
				})
			}
		}
	}

	delete(data, "env")
	delete(data, "envFrom")

	if len(environment) > 0 {
		data["environment"] = environment
	}
	if len(environmentFrom) > 0 {
		data["environmentFrom"] = environmentFrom
	}
}

func (e EnvironmentMapper) ToInternal(data map[string]interface{}) error {
	var environment, env, envFrom []map[string]interface{}
	var valueFrom map[string]interface{}

	if err := convert.ToObj(data["environment"], &environment); err == nil {
		for _, environmentVar := range environment {
			if convert.ToString(environmentVar["value"]) != "" {
				env = append(env, map[string]interface{}{
					"name":  environmentVar["name"],
					"value": environmentVar["value"],
				})
				continue
			}
			if err = convert.ToObj(environmentVar["valueFrom"], &valueFrom); err == nil {
				source := convert.ToString(valueFrom["source"])
				if source == "" {
					continue
				}
				targetKey := convert.ToString(valueFrom["targetKey"])
				sourceKey := convert.ToString(valueFrom["sourceKey"])
				if targetKey == "" {
					targetKey = sourceKey
				}
				switch source {
				case "field":
					env = append(env, map[string]interface{}{
						"name": targetKey,
						"valueFrom": map[string]interface{}{
							"fieldRef": map[string]interface{}{
								"fieldPath": valueFrom["sourceName"],
							},
						},
					})
				case "resource":
					env = append(env, map[string]interface{}{
						"name": targetKey,
						"valueFrom": map[string]interface{}{
							"resourceFieldRef": map[string]interface{}{
								"containerName": valueFrom["sourceName"],
								"resource":      valueFrom["sourceKey"],
								"divisor":       valueFrom["divisor"],
							},
						},
					})
				case "configMap":
					env = append(env, map[string]interface{}{
						"name": targetKey,
						"valueFrom": map[string]interface{}{
							"configMapKeyRef": map[string]interface{}{
								"name":     valueFrom["sourceName"],
								"key":      valueFrom["sourceKey"],
								"optional": valueFrom["optional"],
							},
						},
					})
				case "secret":
					env = append(env, map[string]interface{}{
						"name": targetKey,
						"valueFrom": map[string]interface{}{
							"secretKeyRef": map[string]interface{}{
								"name":     valueFrom["sourceName"],
								"key":      valueFrom["sourceKey"],
								"optional": valueFrom["optional"],
							},
						},
					})
				}
			}
		}
	}
	for _, environmentFrom := range convert.ToMapSlice(data["environmentFrom"]) {
		source := convert.ToString(environmentFrom["source"])
		if source == "" {
			continue
		}
		targetKey := convert.ToString(environmentFrom["targetKey"])
		sourceKey := convert.ToString(environmentFrom["sourceKey"])
		if targetKey == "" && sourceKey == "" {
			switch source {
			case "secret":
				envFrom = append(envFrom, map[string]interface{}{
					"prefix": environmentFrom["prefix"],
					"secretRef": map[string]interface{}{
						"name":     environmentFrom["sourceName"],
						"optional": environmentFrom["optional"],
					},
				})
			case "configMap":
				envFrom = append(envFrom, map[string]interface{}{
					"prefix": environmentFrom["prefix"],
					"configMapRef": map[string]interface{}{
						"name":     environmentFrom["sourceName"],
						"optional": environmentFrom["optional"],
					},
				})
			}
		}
	}

	delete(data, "environment")
	delete(data, "environmentFrom")
	data["env"] = env
	data["envFrom"] = envFrom

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
