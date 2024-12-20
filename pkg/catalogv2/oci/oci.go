package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/hashicorp/go-version"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/errcode"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
)

// maxHelmRepoIndexSize defines what is the max size of helm repo index file we support.
var maxHelmRepoIndexSize = 30 * 1024 * 1024 // 30 MiB

// Chart returns an io.ReadCloser of the chart tar that is requested.
// It uses oras Go library to download the manifest of the OCI artifact
// checks if it is a Helm chart and then return the chart tar layer.
func Chart(credentialSecret *corev1.Secret, chart *repo.ChartVersion, clusterRepoSpec v1.RepoSpec) (io.ReadCloser, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chartURL := chart.URLs[0]

	// Create a new OCIClient
	ociClient, err := NewClient(chartURL, clusterRepoSpec, credentialSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create an OCI client for url %s: %w", chartURL, err)
	}

	// Create an oras repository
	orasRepository, err := ociClient.GetOrasRepository()
	if err != nil {
		return nil, fmt.Errorf("failed to create an OCI repository for url %s: %w", chartURL, err)
	}

	// Download the oci artifact manifest
	memoryStore := memory.New()
	manifest, err := oras.Copy(ctx, orasRepository, ociClient.tag, memoryStore, "", oras.CopyOptions{
		CopyGraphOptions: oras.CopyGraphOptions{
			PreCopy: func(ctx context.Context, desc ocispecv1.Descriptor) error {
				// Download only helm chart related descriptors.
				if desc.MediaType == ocispecv1.MediaTypeImageManifest ||
					desc.MediaType == registry.ChartLayerMediaType {
					// We cannot load huge amounts of data into the memory
					// and so we are defining a limit before fetching.
					if desc.Size > maxHelmChartTarSize {
						return fmt.Errorf("the oci artifact %s has size more than %d which is not supported", chartURL, maxHelmChartTarSize)
					}
					return nil
				}

				return oras.SkipNode
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("unable to oras copy the remote OCI artifact %s: %w", chartURL, err)
	}
	// Fetch the manifest blob of the oci artifact
	manifestBlob, err := content.FetchAll(ctx, memoryStore, manifest)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch the manifest blob of %s: %w", chartURL, err)
	}
	var manifestJSON ocispecv1.Manifest
	err = json.Unmarshal(manifestBlob, &manifestJSON)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal manifest blob of %s: %w", chartURL, err)
	}

	// Check if the oci artifact is of type helm config ?
	if manifest.ArtifactType == registry.ConfigMediaType || manifestJSON.Config.MediaType == registry.ConfigMediaType {
		// Find the layer of chart type and fetch it
		for _, layer := range manifestJSON.Layers {
			if layer.MediaType == registry.ChartLayerMediaType {
				chartTar, err := content.FetchAll(ctx, memoryStore, layer)
				if err != nil {
					return nil, err
				}

				return io.NopCloser(bytes.NewBuffer(chartTar)), nil
			}
		}
	}

	return nil, fmt.Errorf("unable to find the required chart tar file for %s", chartURL)
}

// GenerateIndex creates a Helm repo index from the OCI url provided
// by fetching the repositories and then the tags according to the url.
// Lastly, adds the chart entry to the Helm repo index using the oras library.
func GenerateIndex(ociClient *Client, URL string, credentialSecret *corev1.Secret,
	clusterRepoSpec v1.RepoSpec,
	clusterRepoStatus v1.RepoStatus,
	indexFile *repo.IndexFile) (*repo.IndexFile, error) {
	logrus.Debugf("Generating index for oci clusterrepo URL %s", URL)

	// Checking if the URL specified by the user is a oras repository or not ?
	IsOrasRepository, err := ociClient.IsOrasRepository()
	if err != nil {
		return nil, err
	}

	var maxTag *version.Version
	var chartName string

	// Loop over all the tags and find the latest version
	tagsFunc := func(tags []string) error {
		existingTags := make(map[string]bool)
		for i := len(tags) - 1; i >= 0; i-- {
			existingTags[tags[i]] = true
			// Check if the tag is a valid semver version or not. If yes, then proceed.
			semverTag, err := version.NewVersion(tags[i])
			if err != nil {
				// skipping the tag since it is not semver
				continue
			}

			if maxTag == nil || maxTag.LessThan(semverTag) {
				maxTag = semverTag
			}

			// Add tags into the helm repo index
			if !indexFile.Has(chartName, tags[i]) {
				chartVersion := &repo.ChartVersion{
					Metadata: &chart.Metadata{
						Version: tags[i],
						Name:    chartName,
					},
					URLs: []string{fmt.Sprintf("oci://%s/%s:%s", ociClient.registry, ociClient.repository, tags[i])},
				}
				indexFile.Entries[chartName] = append(indexFile.Entries[chartName], chartVersion)
			}
		}
		var updatedChartVersions []*repo.ChartVersion

		// Only keep the versions that are also present in the /tags/list call
		for _, chartVersion := range indexFile.Entries[chartName] {
			if existingTags[chartVersion.Version] {
				updatedChartVersions = append(updatedChartVersions, chartVersion)
			}
		}
		indexFile.Entries[chartName] = updatedChartVersions
		return nil
	}

	// Loop over all the repositories and fetch the tags
	repositoriesFunc := func(repositories []string) error {
		existingCharts := make(map[string]bool)
		for _, repository := range repositories {
			logrus.Debugf("found repository %s for OCI clusterrepo URL %s", repository, URL)
			// Storing the user provided repository that can be an oras repository or a sub repository.
			userProvidedRepository := ociClient.repository

			// Work on the oci repositories that match with the userProvidedRepository
			if _, found := strings.CutPrefix(repository, ociClient.repository); found {
				ociClient.repository = repository
				orasRepository, err := ociClient.GetOrasRepository()
				if err != nil {
					return fmt.Errorf("failed to create an oras repository for url %s: %w", URL, err)
				}
				chartName = ociClient.repository[strings.LastIndex(ociClient.repository, "/")+1:]
				existingCharts[chartName] = true
				maxTag = nil

				// call tags to get the max tag and update the indexFile
				err = orasRepository.Tags(context.Background(), "", tagsFunc)
				if err != nil {
					return fmt.Errorf("failed to fetch tags for repository %s: %w", URL, err)
				}

				if maxTag != nil {
					ociClient.tag = maxTag.Original()
					err = addToHelmRepoIndex(*ociClient, indexFile, orasRepository)
					if err != nil {
						// Users can have access to only some repositories and not all.
						// So, if pulling the chart fails due to Forbiden error, then skip it.
						var errResp *errcode.ErrorResponse
						if errors.As(err, &errResp) && errResp.StatusCode == http.StatusForbidden {
							delete(indexFile.Entries, chartName)
							logrus.Warnf("failed to add OCI repository %s to helm repo index: %v", ociClient.repository, err)
						} else {
							return fmt.Errorf("failed to add tag %s in OCI repository %s to helm repo index: %w", maxTag.String(), ociClient.repository, err)
						}
					}
				}
			}
			ociClient.repository = userProvidedRepository
		}

		// Only keep the charts that are also present in the /_catalog call
		for chartName := range indexFile.Entries {
			if !existingCharts[chartName] {
				delete(indexFile.Entries, chartName)
			}
		}
		return nil
	}

	// If the user has provided the tag, we simply generate the index with that single oci artifact.
	if ociClient.tag != "" {
		orasRepository, err := ociClient.GetOrasRepository()
		if err != nil {
			return nil, fmt.Errorf("failed to create an oras repository for url %s: %w", URL, err)
		}

		err = addToHelmRepoIndex(*ociClient, indexFile, orasRepository)
		if err != nil {
			return nil, fmt.Errorf("failed to add oci artifact %s in OCI URL %s to Helm repo index: %w", ociClient.repository, URL, err)
		}

		// If the repository is provided with no tag, then we fetch all tags.
	} else if IsOrasRepository {
		orasRepository, err := ociClient.GetOrasRepository()
		if err != nil {
			return nil, fmt.Errorf("failed to create an oras repository for url %s: %w", URL, err)
		}
		chartName = ociClient.repository[strings.LastIndex(ociClient.repository, "/")+1:]

		// call tags to get the max tag and update the indexFile
		err = orasRepository.Tags(context.Background(), "", tagsFunc)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tags for repository %s: %w", URL, err)
		}

		if maxTag != nil {
			ociClient.tag = maxTag.Original()

			// fetch the chart.yaml for the latest tag and add it to the index.
			err = addToHelmRepoIndex(*ociClient, indexFile, orasRepository)
			if err != nil {
				return indexFile, fmt.Errorf("failed to add tag %s in OCI repository %s to helm repo index: %w", maxTag.String(), ociClient.repository, err)
			}
		}
		// If no repository and tag is provided, we fetch
		// all repositories and then tags associated.
	} else {
		orasRegistry, err := ociClient.GetOrasRegistry()
		if err != nil {
			return nil, fmt.Errorf("failed to create an oras registry for %s: %w", ociClient.registry, err)
		}

		// Fetch all repositories
		err = orasRegistry.Repositories(context.Background(), "", repositoriesFunc)
		if err != nil {
			return indexFile, fmt.Errorf("failed to fetch repositories for %s: %w", URL, err)
		}
	}

	return indexFile, nil
}

// addToHelmRepoIndex adds the helmchart aka oras repository to the helm repo index
func addToHelmRepoIndex(ociClient Client, indexFile *repo.IndexFile, orasRepository *remote.Repository) (err error) {
	ociURL := fmt.Sprintf("%s/%s:%s", ociClient.registry, ociClient.repository, ociClient.tag)
	filePath := ""
	var indexFileBytes []byte

	// Delete the temporary file created to store the helm chart.
	defer func() {
		if filePath != "" {
			err2 := os.Remove(filePath)
			if err == nil {
				err = err2
			}
		}
	}()
	// The chartname is always the last part of the repository
	// Helm codebase also says the same thing https://github.com/helm/helm/blob/main/pkg/pusher/ocipusher.go#L88
	chartName := ociClient.repository[strings.LastIndex(ociClient.repository, "/")+1:]

	// Check if the repository and tag are not already present in the indexFile
	// If it is already present, skip adding it to the indexFile.
	for _, entry := range indexFile.Entries[chartName] {
		if entry.Metadata.Name == chartName && entry.Version == ociClient.tag && entry.Digest != "" {
			logrus.Debugf("skip adding chart %s version %s since it is already present in the index", chartName, ociClient.tag)
			return
		}
	}

	// Fetch the helm chart tar to get the Chart.yaml
	filePath, err = ociClient.fetchChart(orasRepository)
	if err != nil {
		err = fmt.Errorf("failed to fetch the helm chart %s: %w", ociURL, err)
		return
	}

	// We load index into memory and so we should set a limit
	// to the size of the index file that is being created.
	indexFileBytes, err = json.Marshal(indexFile)
	if err != nil {
		return
	}
	if len(indexFileBytes) > maxHelmRepoIndexSize {
		err = fmt.Errorf("there are a lot of charts inside this oci URL %s which is making the index larger than %d", ociURL, maxHelmRepoIndexSize)
		return
	}

	// Remove the entry from the indexfile since the next function addToIndex
	// will add. This is done to avoid duplication.
	for index, entry := range indexFile.Entries[chartName] {
		if entry.Metadata.Name == chartName && entry.Version == ociClient.tag {
			indexFile.Entries[chartName] = append(indexFile.Entries[chartName][:index], indexFile.Entries[chartName][index+1:]...)
		}
	}

	// Add the chart to the index
	err = ociClient.addToIndex(indexFile, filePath)
	if err != nil {
		err = fmt.Errorf("unable to add helm chart %s to index: %w", ociURL, err)
	}

	return
}
