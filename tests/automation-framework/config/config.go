package config

import (
	"io/ioutil"
	"os"

	"github.com/creasty/defaults"
	"gopkg.in/yaml.v2"
)

// LoadConfig reads the file defined by  the `CATTLE_TEST_CONFIG` environment variable and loads the object found at the given key onto the given configuration reference.
// The functions takes a pointer of the object.
func LoadConfig(key string, config interface{}) {
	configPath := os.Getenv("CATTLE_TEST_CONFIG")
	var configMap map[string]interface{}

	configContents, err := ioutil.ReadFile(configPath)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(configContents, &configMap)
	if err != nil {
		panic(err)
	}

	configObject := configMap[key]
	yamlEncodedConfigObject, err := yaml.Marshal(configObject)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(yamlEncodedConfigObject, config)
	if err != nil {
		panic(err)
	}

	if err := defaults.Set(config); err != nil {
		panic(err)
	}

}
