package appco

import (
	"fmt"
	"net/http"

	"gopkg.in/yaml.v3"
)

const DefaultConfigURL = "https://raw.githubusercontent.com/rancher/artifact-mirror/master/config.yaml"

func loadArtifacts() ([]*Artifact, error) {
	resp, err := http.Get(DefaultConfigURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("appco: failed to fetch config: %s", resp.Status)
	}

	var cfg Config
	if err := yaml.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, err
	}

	return cfg.Artifacts, nil
}
