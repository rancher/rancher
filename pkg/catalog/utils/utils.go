package utils

import (
	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

// Config holds libcompose top level configuration
type Config struct {
	Version  string                 `yaml:"version,omitempty"`
	Services RawServiceMap          `yaml:"services,omitempty"`
	Volumes  map[string]interface{} `yaml:"volumes,omitempty"`
	Networks map[string]interface{} `yaml:"networks,omitempty"`
}

// RawService is represent a Service in map form unparsed
type RawService map[string]interface{}

// RawServiceMap is a collection of RawServices
type RawServiceMap map[string]RawService

// CreateConfig unmarshals bytes to config and creates config based on version
func CreateConfig(bytes []byte) (*Config, error) {
	var config Config
	if err := yaml.Unmarshal(bytes, &config); err != nil {
		return nil, err
	}

	if config.Version != "2" {
		var baseRawServices RawServiceMap
		if err := yaml.Unmarshal(bytes, &baseRawServices); err != nil {
			return nil, err
		}
		config.Services = baseRawServices
	}

	if config.Volumes == nil {
		config.Volumes = make(map[string]interface{})
	}
	if config.Networks == nil {
		config.Networks = make(map[string]interface{})
	}

	return &config, nil
}

// Convert converts a struct (src) to another one (target) using yaml marshalling/unmarshalling.
// If the structure are not compatible, this will throw an error as the unmarshalling will fail.
func Convert(src, target interface{}) error {
	newBytes, err := yaml.Marshal(src)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(newBytes, target)
	if err != nil {
		logrus.Errorf("Failed to unmarshall: %v\n%s", err, string(newBytes))
	}
	return err
}

func Contains(collection []string, key string) bool {
	for _, value := range collection {
		if value == key {
			return true
		}
	}

	return false
}
