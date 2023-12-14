package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/creasty/defaults"
	"sigs.k8s.io/yaml"
)

// ConfigEnvironmentKey is a const that stores cattle config's environment key.
const ConfigEnvironmentKey = "CATTLE_TEST_CONFIG"

// LoadConfig reads the file defined by  the `CATTLE_TEST_CONFIG` environment variable and loads the object found at the given key onto the given configuration reference.
// The functions takes a pointer of the object.
func LoadConfig(key string, config interface{}) {
	configPath := os.Getenv(ConfigEnvironmentKey)

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
	configPath := os.Getenv(ConfigEnvironmentKey)

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

// LoadAndUpdateConfig is function that loads and updates the CATTLE_TEST_CONFIG yaml/json that the framework uses,
// accepts a func to update the configuration file.
func LoadAndUpdateConfig(key string, config any, updateFunc func()) {
	LoadConfig(key, config)

	updateFunc()

	UpdateConfig(key, config)
}

// WriteConfig writes a CATTLE_TEST_CONFIG config file when one is not previously written.
func WriteConfig(key string, config interface{}) error {
	configPath := os.Getenv("CATTLE_TEST_CONFIG")
	if configPath == "" {
		return errors.New("cannot write config because environment variable CATTLE_TEST_CONFIG is not set")
	}

	all := map[string]interface{}{}
	all[key] = config

	yamlConfig, err := yaml.Marshal(all)
	if err != nil {
		return fmt.Errorf("error marshalling config as YAML: %w", err)
	}

	err = os.WriteFile(configPath, yamlConfig, 0644)
	if err != nil {
		return fmt.Errorf("error writing config to file: %w", err)
	}

	return nil
}
