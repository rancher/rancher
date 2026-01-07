package appco

import (
	img "github.com/rancher/rancher/pkg/image"
)

func CollectArtifacts(rancherVersion string) ([]string, error) {
	artifacts, err := loadArtifacts()
	if err != nil {
		return nil, err
	}

	allowed, err := loadAppCoAllowList(rancherVersion)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, a := range artifacts {
		if !isCorrectAppCoArtifact(*a) {
			continue
		}

		for _, u := range a.URLs() {
			if _, ok := allowed[u]; ok {
				result = append(result, u)
			}
		}
	}

	for _, image := range result {
		img.Mirrors[image] = image
	}

	return result, nil
}
