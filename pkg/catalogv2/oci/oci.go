package oci

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Masterminds/semver"
	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	corev1controllers "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
)

// maxHelmRepoIndexSize defines what is the max size of helm repo index file we support.
const maxHelmRepoIndexSize int = 30 * 1024 * 1024 // 30 MiB

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
func GenerateIndex(URL string, credentialSecret *corev1.Secret, clusterRepoSpec v1.RepoSpec, clusterRepoStatus v1.RepoStatus, configMapCache corev1controllers.ConfigMapCache) (*repo.IndexFile, error) {
	logrus.Debugf("Generating index for oci clusterrepo URL %s", URL)

	// Create a new oci client
	ociClient, err := NewClient(URL, clusterRepoSpec, credentialSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create an OCI client for url %s: %w", URL, err)
	}

	indexFile := repo.NewIndexFile()

	// if the index file configmap already exists, use it instead of creating a new one.
	if clusterRepoStatus.IndexConfigMapName != "" && clusterRepoSpec.URL == clusterRepoStatus.URL {
		configMap, err := configMapCache.Get(clusterRepoStatus.IndexConfigMapNamespace, clusterRepoStatus.IndexConfigMapName)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch existing configmap of indexfile for URL %s", URL)
		}

		data, err := readBytes(configMapCache, configMap)
		if err != nil {
			return nil, fmt.Errorf("failed to read bytes of existing configmap for URL %s", URL)
		}
		gz, err := gzip.NewReader(bytes.NewBuffer(data))
		if err != nil {
			return nil, err
		}
		defer gz.Close()

		data, err = io.ReadAll(gz)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(data, indexFile); err != nil {
			return nil, err
		}
	}

	var orasRepository *remote.Repository
	chartName := ""

	// Loop over all the tags and add to helm repo index
	tagsFunc := func(tags []string) error {
		// We don't need to switch from + to _ like helm does https://github.com/helm/helm/blob/e81f6140ddb22bc99a08f7409522a8dbe5338ee3/pkg/registry/util.go#L123-L142
		// since we are pulling the chart and not pushing the chart.+ in version is not accepted in OCI registries.
		for _, tag := range tags {
			// Check if the tag is a valid semver version or not. If yes, then proceed.
			if _, err := semver.NewVersion(tag); err == nil {
				logrus.Debugf("found a tag %s for the repository %s for OCI clusterrepo URL %s", tag, ociClient.repository, URL)
				ociURL := fmt.Sprintf("%s/%s:%s", ociClient.registry, ociClient.repository, ociClient.tag)
				ociClient.tag = tag

				orasRepository, err = ociClient.GetOrasRepository()
				if err != nil {
					return fmt.Errorf("failed to create an oras repository for url %s: %w", ociURL, err)
				}

				err = addToHelmRepoIndex(*ociClient, chartName, indexFile, orasRepository)
				if err != nil {
					return fmt.Errorf("failed to add chartName %s in OCI URL %s to helm repo index: %w", chartName, ociURL, err)
				}
			}
		}
		return nil
	}

	// Loop over all the repositories and fetch the tags
	repositoriesFunc := func(repositories []string) error {
		for _, repository := range repositories {
			logrus.Debugf("found repository %s for OCI clusterrepo URL %s", repository, URL)

			// Storing the user provided repository that can be an oras repository or a sub repository.
			userProvidedRepository := ociClient.repository

			// Work on the oci repositories that match with the userProvidedRepository
			if subRepo, found := strings.CutPrefix(repository, ociClient.repository); found {
				// We only proceed for the following two conditions
				// 1: If the entire word is cut such as `charts` in oci://example.com/charts/etcd, not `cha` in `oci://example.com/charts/etcd`.
				// 2: there is nothing to cut, which means userprovidedRepository is an orasRepository.
				if strings.HasPrefix(subRepo, "/") || repository == subRepo {
					chartName = strings.Trim(subRepo, "/")

					// We need to replace the repo given by the user with the full repository for fetching the helm chart.
					ociClient.repository = repository
					ociURL := fmt.Sprintf("%s/%s", ociClient.registry, ociClient.repository)

					orasRepository, err = ociClient.GetOrasRepository()
					if err != nil {
						return fmt.Errorf("failed to create an oras repository for url %s: %w", ociURL, err)
					}

					err = orasRepository.Tags(context.Background(), "", tagsFunc)
					if err != nil {
						return fmt.Errorf("failed to fetch tags for repository %s: %w", ociURL, err)
					}
				}
			}
			ociClient.repository = userProvidedRepository
		}
		return nil
	}

	IsOrasRepository, err := ociClient.IsOrasRepository()
	if err != nil {
		return nil, err
	}

	// If the user has provided the tag, we simply generate the index with that single oci artifact.
	if ociClient.tag != "" {
		orasRepository, err = ociClient.GetOrasRepository()
		if err != nil {
			return nil, fmt.Errorf("failed to create an oras repository for url %s: %w", URL, err)
		}

		err = addToHelmRepoIndex(*ociClient, chartName, indexFile, orasRepository)
		if err != nil {
			return nil, fmt.Errorf("failed to add chartName %s in OCI URL %s to Helm repo index: %w", chartName, URL, err)
		}

		// If the repository is provided with no tag, then we fetch all tags.
	} else if IsOrasRepository {
		orasRepository, err = ociClient.GetOrasRepository()
		if err != nil {
			return nil, fmt.Errorf("failed to create an oras repository for url %s: %w", URL, err)
		}

		err = orasRepository.Tags(context.Background(), "", tagsFunc)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tags for repository %s: %w", URL, err)
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
			return nil, fmt.Errorf("failed to fetch repositories for %s: %w", URL, err)
		}
	}

	return indexFile, nil
}

// addToHelmRepoIndex adds the helmchart aka oras repository to the helm repo index
func addToHelmRepoIndex(ociClient Client, chartName string, indexFile *repo.IndexFile, orasRepository *remote.Repository) error {
	ociURL := fmt.Sprintf("%s/%s:%s", ociClient.registry, ociClient.repository, ociClient.tag)
	filePath := ""
	var err error

	// Delete the file
	defer func() {
		if filePath != "" {
			err = os.Remove(filePath)
		}
	}()

	// If chartname is empty, then just take the last part of the repository
	// If chartname is not empty, then use the chartname
	if chartName == "" {
		chartName = ociClient.repository[strings.LastIndex(ociClient.repository, "/")+1:]
	}
	for _, entry := range indexFile.Entries[chartName] {
		if entry.Metadata.Name == chartName && entry.Version == ociClient.tag {
			logrus.Debugf("skip adding chart %s version %s since it is already present in the index", chartName, ociClient.tag)
			return nil
		}
	}

	// Fetch the helm chart
	filePath, isHelmChart, err := ociClient.fetchChart(orasRepository)
	if err != nil {
		return fmt.Errorf("failed to fetch an helm chart %s: %w", ociURL, err)
	}

	// We load index into memory and so we should set a limit
	// to the size of the index file that is being created.
	indexFileBytes, err := json.Marshal(indexFile)
	if err != nil {
		return err
	}

	if len(indexFileBytes) > maxHelmRepoIndexSize {
		return fmt.Errorf("there are a lot of charts inside this oci URL %s which is making the index larger than %d", ociURL, maxHelmRepoIndexSize)
	}

	// Add to the index if it is a helm chart
	if isHelmChart {
		err = ociClient.addToIndex(indexFile, chartName, filePath)
		if err != nil {
			return fmt.Errorf("unable to add helm chart %s to index: %w", ociURL, err)
		}
	}

	return err
}

// readBytes reads data from the chain of helm repo index configmaps.
func readBytes(configMapCache corev1controllers.ConfigMapCache, cm *corev1.ConfigMap) ([]byte, error) {
	var (
		bytes = cm.BinaryData["content"]
		err   error
	)

	for {
		next := cm.Annotations["catalog.cattle.io/next"]
		if next == "" {
			break
		}
		cm, err = configMapCache.Get(cm.Namespace, next)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, cm.BinaryData["content"]...)
	}

	return bytes, nil
}
