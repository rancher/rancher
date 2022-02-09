package utils

import (
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	helmlib "github.com/rancher/rancher/pkg/helm"
)

const (
	CatalogExternalIDFormat = "catalog://?catalog=%s&template=%s&version=%s"
	SystemLibraryName       = "system-library"
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

func GetCatalogImageCacheName(catalogName string) string {
	return fmt.Sprintf("%s-catalog-image-list", catalogName)
}

func GetCatalogChartPath(catalog *v3.Catalog, bundledMode bool) (string, error) {
	if bundledMode {
		switch catalog.Name {
		case "helm3-library", "library", "system-library":
			return filepath.Join(helmlib.InternalCatalog, catalog.Name), nil
		case "rancher-charts", "rancher-partner-charts", "rancher-rke2-charts":
			return filepath.Join(helmlib.InternalCatalog, "v2", catalog.Name), nil
		default:
			return "", fmt.Errorf("cannot find bundled catalog chart path for catalog %s", catalog.Name)
		}
	}
	return filepath.Join(helmlib.CatalogCache, helmlib.CatalogSHA256Hash(catalog)), nil
}
