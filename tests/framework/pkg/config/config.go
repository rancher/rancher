package config

import (
	"os"

	"github.com/creasty/defaults"
	"gopkg.in/yaml.v2"
)

// LoadConfig reads the file defined by  the `CATTLE_TEST_CONFIG` environment variable and loads the object found at the given key onto the given configuration reference.
// The functions takes a pointer of the object.
func LoadConfig(key string, config interface{}) {
	configPath := os.Getenv("CATTLE_TEST_CONFIG")

	if configPath == "" {
		yaml.Unmarshal([]byte("{}"), config)
		return
	}

	allString, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}

	var all map[string]interface{}
	err = yaml.Unmarshal(allString, &all)
	if err != nil {
		panic(err)
	}

	scoped := all[key]
	scopedString, err := yaml.Marshal(scoped)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(scopedString, config)
	if err != nil {
		panic(err)
	}

	if err := defaults.Set(config); err != nil {
		panic(err)
	}

}

// UpdateConfig is function that updates the CATTLE_TEST_CONFIG yaml/json that the framework uses.
func UpdateConfig(key string, config interface{}) {
	configPath := os.Getenv("CATTLE_TEST_CONFIG")

	if configPath == "" {
		yaml.Unmarshal([]byte("{}"), config)
		return
	}

	// Read json buffer from jsonFile
	byteValue, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}

	// We have known the outer json object is a map, so we define result as map.
	// otherwise, result could be defined as slice if outer is an array
	var result map[string]interface{}
	err = yaml.Unmarshal(byteValue, &result)
	if err != nil {
		panic(err)
	}

	result[key] = config

	yamlConfig, err := yaml.Marshal(result)

	if err != nil {
		panic(err)
	}

	err = os.WriteFile(configPath, yamlConfig, 0644)
	if err != nil {
		panic(err)
	}
}
