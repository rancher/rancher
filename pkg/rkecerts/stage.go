package rkecerts

import (
	"context"
	"os"

	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func Stage(rkeConfig *v3.RancherKubernetesEngineConfig) (*Bundle, error) {
	bundle, err := Load()
	if os.IsNotExist(err) {
		bundle, err = Generate(context.Background(), rkeConfig)
		if err != nil {
			return nil, err
		}
		if err := bundle.Save(); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return bundle, bundle.Explode()
}
