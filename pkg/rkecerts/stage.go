package rkecerts

import (
	"os"

	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func Stage(rkeConfig *v3.RancherKubernetesEngineConfig) (*Bundle, error) {
	bundle, err := LoadLocal()
	if os.IsNotExist(err) {
		bundle, err = Generate(rkeConfig)
		if err != nil {
			return nil, err
		}
		if err := bundle.SaveLocal(); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return bundle, bundle.Explode()
}
