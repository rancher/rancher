package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	helmregistry "helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/Masterminds/semver"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
)

// Chart returns an io.ReadCloser of the chart tar that is requested.
// It uses oras go library to download the manifest of the oci artifiact
// checks if it is a helm chart and then return the chart tar layer.
func Chart(credentialSecret *corev1.Secret, chart *repo.ChartVersion, URL string) (io.ReadCloser, error) {
	if len(chart.URLs) == 0 {
		return nil, fmt.Errorf("failed to find chartName %s version %s: %w", chart.Name, chart.Version, validation.NotFound)
	}

	// Create a new OCIClient
	ociClient, err := NewOCIClient(URL)
	if err != nil {
		return nil, fmt.Errorf("failed to create an oci client for url %s: %s", URL, err.Error())
	}

	// Create an oras repository
	orasRepository, err := ociClient.getOrasRepository(credentialSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create an oci repository for url %s: %s", fmt.Sprintf("%s/%s", ociClient.registry, ociClient.repository), err.Error())
	}

	// Download the oci artifact manifest
	memoryStore := memory.New()
	manifest, err := oras.Copy(context.Background(), orasRepository, ociClient.tag, memoryStore, "", oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("unable to oras copy the remote oci artifact %s:%s", fmt.Sprintf("%s/%s:%s", ociClient.registry, ociClient.repository, ociClient.tag), err.Error())
	}

	// Fetch the manifest blob of the oci artifact
	manifestblob, err := content.FetchAll(context.Background(), memoryStore, manifest)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch the manifest blob of %s:%s", fmt.Sprintf("%s/%s:%s", ociClient.registry, ociClient.repository, ociClient.tag), err.Error())
	}

	var manifestJson ocispec.Manifest
	err = json.Unmarshal(manifestblob, &manifestJson)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal manifest blob of %s:%s", fmt.Sprintf("%s/%s:%s", ociClient.registry, ociClient.repository, ociClient.tag), err.Error())
	}

	// Check if the oci artifact is of type helm config ?
	if manifest.ArtifactType == helmregistry.ConfigMediaType || manifestJson.Config.MediaType == helmregistry.ConfigMediaType {
		// find the layer of chart type and fetch it
		for _, layer := range manifestJson.Layers {
			if layer.MediaType == helmregistry.ChartLayerMediaType {

				chartTar, err := content.FetchAll(context.Background(), memoryStore, layer)
				if err != nil {
					return nil, err
				}

				return io.NopCloser(bytes.NewBuffer(chartTar)), nil
			}
		}
	}

	return nil, fmt.Errorf("unable to find the required chart tar file for %s", fmt.Sprintf("%s/%s:%s", ociClient.registry, ociClient.repository, ociClient.tag))
}

// GenerateIndex creates an helm repo index from the oci url provided
// by fetching the repositories or tags according to the url and
// untaring the oci artifact chart and then adding the chart entry to the
// helm repo index using the oras go library.
func GenerateIndex(URL string, credentialSecret *corev1.Secret) (*repo.IndexFile, error) {

	// Create a new oci client
	ociClient, err := NewOCIClient(URL)
	if err != nil {
		return nil, fmt.Errorf("failed to create an oci client for url %s: %s", URL, err.Error())
	}

	// Create an empty helm repo index file
	indexFile := repo.NewIndexFile()

	var orasRepository *remote.Repository
	chartName := ""

	// Loop over tags function
	tagsFunc := func(tags []string) error {
		for _, tag := range tags {
			if _, err := semver.NewVersion(tag); err == nil {
				logrus.Debugf("found tag %s", tag)

				ociClient.tag = tag

				// Fetch the helm chart
				filePath, isHelmChart, err := ociClient.fetchChart(orasRepository)
				if err != nil {
					return fmt.Errorf("failed to fetch an helm chart %s: %s", fmt.Sprintf("%s/%s:%s", ociClient.registry, ociClient.repository, ociClient.tag), err.Error())
				}

				// Add to the index
				if isHelmChart {
					err = ociClient.addToIndex(indexFile, chartName, filePath)
					if err != nil {
						return fmt.Errorf("unable to add helm chart %s to index: %s", fmt.Sprintf("%s/%s:%s", ociClient.registry, ociClient.repository, ociClient.tag), err.Error())
					}
				}
			}
		}
		return nil
	}

	// Loop over repositories function
	repositoriesFunc := func(repositories []string) error {
		for _, repository := range repositories {
			logrus.Debugf("found repository %s", repository)
			tempRepository := ociClient.repository

			if subRepo, found := strings.CutPrefix(repository, ociClient.repository); found {

				chartName = strings.Trim(subRepo, "/")
				ociClient.repository = repository

				// Create an oras repository
				orasRepository, err = ociClient.getOrasRepository(credentialSecret)
				if err != nil {
					return fmt.Errorf("failed to create an oras repository for url %s: %s", fmt.Sprintf("%s/%s", ociClient.registry, ociClient.repository), err.Error())
				}

				// Fetch all tags
				err = orasRepository.Tags(context.Background(), "", tagsFunc)
				if err != nil {
					return fmt.Errorf("failed to fetch tags for repository %s: %s", fmt.Sprintf("%s/%s", ociClient.registry, ociClient.repository), err.Error())
				}
			}
			ociClient.repository = tempRepository
		}
		return nil
	}

	ok, err := ociClient.IsOrasRepository(credentialSecret)
	if err != nil {
		return nil, err
	}

	// The tag is not empty, so we fetch the chart and
	// create a helm repo index for that chart.
	if ociClient.tag != "" {
		// Create an oras repository
		orasRepository, err = ociClient.getOrasRepository(credentialSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to create an oras repository for url %s: %s", fmt.Sprintf("%s/%s", ociClient.registry, ociClient.repository), err.Error())
		}

		// Fetch the helm chart
		filePath, isHelmChart, err := ociClient.fetchChart(orasRepository)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch an helm chart %s: %s", fmt.Sprintf("%s/%s:%s", ociClient.registry, ociClient.repository, ociClient.tag), err.Error())
		}

		// Add to the index
		if isHelmChart {
			err = ociClient.addToIndex(indexFile, chartName, filePath)
			if err != nil {
				return nil, fmt.Errorf("unable to add helm chart %s to index: %s", fmt.Sprintf("%s/%s:%s", ociClient.registry, ociClient.repository, ociClient.tag), err.Error())
			}
		}

		// If the full repository is provided, then we fetch all tas
		// fetch all tags and check if the tag is a helm chart ? and then
		// add that helm chart entry to the helm repo index.
	} else if ok {
		// Create an oras repository
		orasRepository, err = ociClient.getOrasRepository(credentialSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to create an oras repository for url %s: %s", fmt.Sprintf("%s/%s", ociClient.registry, ociClient.repository), err.Error())
		}

		// Fetch all tags
		err = orasRepository.Tags(context.Background(), "", tagsFunc)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tags for repository %s: %s", fmt.Sprintf("%s/%s", ociClient.registry, ociClient.repository), err.Error())
		}

		// The last case where sub repository is provided
		// and we fetch all repositories with sub repository
		// as prefix and fetch all tags and add to helm repo index.
	} else {
		// Create an oras registry
		orasRegistry, err := ociClient.getOrasRegistry(credentialSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to create an oras registry for %s: %s", ociClient.registry, err.Error())
		}

		// Fetch all repositories
		err = orasRegistry.Repositories(context.Background(), "", repositoriesFunc)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch repositories for %s: %s", fmt.Sprintf("%s/%s", ociClient.registry, ociClient.repository), err.Error())
		}
	}

	return indexFile, nil
}
