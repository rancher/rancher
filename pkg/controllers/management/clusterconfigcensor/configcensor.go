package clusterconfigcensor

import (
	"fmt"
	"strings"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
)

func NewConfigCensor(lister v3.KontainerDriverLister, dynamicSchemaLister v3.DynamicSchemaLister) *ConfigCensor {
	return &ConfigCensor{
		KontainerDriverLister: lister,
		DynamicSchemaLister:   dynamicSchemaLister,
	}
}

type ConfigCensor struct {
	KontainerDriverLister v3.KontainerDriverLister
	DynamicSchemaLister   v3.DynamicSchemaLister
}

func (c *ConfigCensor) CensorGenericEngineConfig(input v3.ClusterSpec) (v3.ClusterSpec, error) {
	if input.GenericEngineConfig == nil {
		// nothing to do
		return input, nil
	}

	config := copyMap(*input.GenericEngineConfig)
	driverName, ok := config["driverName"].(string)
	if !ok {
		// can't figure out driver type so blank out the whole thing
		logrus.Warnf("cluster %v has a generic engine config but no driver type field; can't hide password "+
			"fields so removing the entire config", input.DisplayName)
		input.GenericEngineConfig = nil
		return input, nil
	}

	driver, err := c.KontainerDriverLister.Get("", driverName)
	if err != nil {
		return v3.ClusterSpec{}, err
	}

	var schemaName string
	if driver.Spec.BuiltIn {
		schemaName = driver.Status.DisplayName + "Config"
	} else {
		schemaName = driver.Status.DisplayName + "EngineConfig"
	}

	kontainerDriverSchema, err := c.DynamicSchemaLister.Get("", strings.ToLower(schemaName))
	if err != nil {
		return v3.ClusterSpec{}, fmt.Errorf("error getting dynamic schema %v", err)
	}

	for key := range config {
		field := kontainerDriverSchema.Spec.ResourceFields[key]
		if field.Type == "password" {
			delete(config, key)
		}
	}

	input.GenericEngineConfig = &config
	return input, nil
}

func copyMap(toCopy v3.MapStringInterface) v3.MapStringInterface {
	newMap := v3.MapStringInterface{}

	for k, v := range toCopy {
		newMap[k] = v
	}

	return newMap
}
