package main

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
)

type dummyProvider struct {
	settings map[string]string
}

func (d *dummyProvider) Get(name string) string {
	return ""
}

func (d *dummyProvider) Set(name, value string) error {
	return nil
}

func (d *dummyProvider) SetIfUnset(name, value string) error {
	return nil
}

func (d *dummyProvider) SetAll(settings map[string]settings.Setting) error {
	for name := range settings {
		key := "CATTLE_" + strings.ToUpper(strings.Replace(name, "-", "_", -1)) + "_DEFAULT"
		d.settings[key] = name
	}
	return nil
}

/*
The goal of this function is to fetch the default value of settings
from environment variable and return as json to stdout.
The return json should be set to the building parameters
`pkg/settings.InjectDefaults` of Rancher and `InjectDefaults` will
be loaded and override the default value of the settings when Rancher
is initializing.
*/
func main() {
	provider := &dummyProvider{
		settings: map[string]string{},
	}
	settings.SetProvider(provider)
	output := map[string]string{}
	for key, name := range provider.settings {
		value := os.Getenv(key)
		if value == "" {
			continue
		}
		output[name] = value
	}
	data, _ := json.Marshal(output)
	os.Stdout.Write(data)
}
