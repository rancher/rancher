package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"helm.sh/helm/v3/pkg/chart/loader"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/helm/pkg/provenance"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	helmregistry "helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry"
	orasRegistryRemote "oras.land/oras-go/v2/registry/remote"
	orasRegistryAuth "oras.land/oras-go/v2/registry/remote/auth"
)

type OCIClient struct {
	URL        string
	repository string
	registry   string
	tag        string
}

// NewOCIClient returns a new OCIClient along
// parsing the URL provided.
func NewOCIClient(url string) (*OCIClient, error) {
	ociClient := &OCIClient{
		URL: url,
	}

	err := ociClient.parseURL()
	if err != nil {
		return nil, err
	}

	return ociClient, nil
}

func (o *OCIClient) parseURL() error {
	url := strings.TrimSuffix(o.URL, "/")
	url = strings.TrimPrefix(url, "oci://")

	if strings.Contains(url, "/") {
		var ref registry.Reference
		ref, err := registry.ParseReference(url)
		if err != nil {
			return fmt.Errorf("failed to parse OCI URL '%s' value: %w", o.URL, err)
		}
		o.registry = ref.Registry
		o.repository = ref.Repository
		o.tag = ref.Reference
	} else {
		o.registry = url
	}

	return nil
}

func (o *OCIClient) fetchChart(orasRepository *orasRegistryRemote.Repository) (string, bool, error) {

	memoryStore := memory.New()
	manifest, err := oras.Copy(context.Background(), orasRepository, o.tag, memoryStore, "", oras.DefaultCopyOptions)
	if err != nil {
		return "", false, fmt.Errorf("unable to oras copy the remote oci artifact %s:%s", fmt.Sprintf("%s/%s:%s", o.registry, o.repository, o.tag), err.Error())
	}

	// Fetch the manifest content of the oci artifact
	manifestblob, err := content.FetchAll(context.Background(), memoryStore, manifest)
	if err != nil {
		return "", false, fmt.Errorf("unable to fetch the manifest blob of %s:%s", fmt.Sprintf("%s/%s:%s", o.registry, o.repository, o.tag), err.Error())
	}

	var manifestJson ocispec.Manifest
	err = json.Unmarshal(manifestblob, &manifestJson)
	if err != nil {
		return "", false, fmt.Errorf("unable to unmarshal manifest blob of %s:%s", fmt.Sprintf("%s/%s:%s", o.registry, o.repository, o.tag), err.Error())
	}

	// Create a temp file to store the helm chart tar
	tempFile, err := os.CreateTemp("", "helm-")
	if err != nil {
		return "", false, fmt.Errorf("unable to create temp file for storing the oci artifact")
	}

	// Check if the oci artifact is of type helm config ?
	if manifest.ArtifactType == helmregistry.ConfigMediaType || manifestJson.Config.MediaType == helmregistry.ConfigMediaType {
		// find the layer of chart type and fetch it
		for _, layer := range manifestJson.Layers {
			if layer.MediaType == helmregistry.ChartLayerMediaType {

				chartTar, err := content.FetchAll(context.Background(), memoryStore, layer)
				if err != nil {
					return "", false, fmt.Errorf("unable to fetch chart blob of %s:%s", fmt.Sprintf("%s/%s:%s", o.registry, o.repository, o.tag), err.Error())
				}

				err = os.WriteFile(tempFile.Name(), chartTar, 0644)
				if err != nil {
					return "", false, fmt.Errorf("unable to write chart %s into file %s:%s", fmt.Sprintf("%s/%s:%s", o.registry, o.repository, o.tag), tempFile.Name(), err.Error())
				}

				return tempFile.Name(), true, nil
			}
		}
	}

	return "", false, nil
}

func (o *OCIClient) getOrasRegistry(credentialSecret *corev1.Secret) (*orasRegistryRemote.Registry, error) {
	username := ""
	password := ""

	if credentialSecret != nil {
		if credentialSecret.Type == corev1.SecretTypeBasicAuth {
			username, password = string(credentialSecret.Data[corev1.BasicAuthUsernameKey]), string(credentialSecret.Data[corev1.BasicAuthPasswordKey])
			if len(password) == 0 || len(username) == 0 {
				return nil, fmt.Errorf("username or password is empty")
			}
		} else {
			return nil, fmt.Errorf("only basicauth credential is supported")
		}
	}

	orasRegistry, err := orasRegistryRemote.NewRegistry(o.registry)
	if err != nil {
		return nil, err
	}

	orasRegistry.Client = &orasRegistryAuth.Client{
		Header: orasRegistryAuth.DefaultClient.Header.Clone(),
		Cache:  orasRegistryAuth.DefaultCache,
		Credential: func(ctx context.Context, reg string) (orasRegistryAuth.Credential, error) {
			return orasRegistryAuth.Credential{
				Username: username,
				Password: password,
			}, nil
		},
	}

	return orasRegistry, nil
}

func (o *OCIClient) getOrasRepository(credentialSecret *corev1.Secret) (*orasRegistryRemote.Repository, error) {

	username := ""
	password := ""

	if credentialSecret != nil {
		if credentialSecret.Type == corev1.SecretTypeBasicAuth {
			username, password = string(credentialSecret.Data[corev1.BasicAuthUsernameKey]), string(credentialSecret.Data[corev1.BasicAuthPasswordKey])
			if len(password) == 0 && len(username) == 0 {
				return nil, fmt.Errorf("username or password is empty")
			}
		} else {
			return nil, fmt.Errorf("only basicauth credential is supported")
		}
	}

	repository, err := orasRegistryRemote.NewRepository(fmt.Sprintf("%s/%s", o.registry, o.repository))
	if err != nil {
		return nil, fmt.Errorf("failed to create oras repository for repository %s: %s", o.repository, err.Error())
	}

	repository.Client = &orasRegistryAuth.Client{
		Header: orasRegistryAuth.DefaultClient.Header.Clone(),
		Cache:  orasRegistryAuth.DefaultCache,
		Credential: func(ctx context.Context, reg string) (orasRegistryAuth.Credential, error) {
			return orasRegistryAuth.Credential{
				Username: username,
				Password: password,
			}, nil
		},
	}

	return repository, nil
}

// addToIndex adds the given helm chart entry into the helm repo index provided.
func (o *OCIClient) addToIndex(indexFile *repo.IndexFile, chartName, fileName string) error {

	// Load the Chart into chart golang struct.
	chart, err := loader.Load(fileName)
	if err != nil {
		return fmt.Errorf("failed to load the chart %s: %s", chart.Metadata.Name, err.Error())
	}

	// If chartName is not empty, then replace chart name with the chartName
	// since a chartName can exist in multiple helm repositories in a oci registry.
	// and replacing with customisedChartName will make the chart name unqiue in helm repo index.
	if chartName != "" {
		chart.Metadata.Name = chartName
	}

	// Generate the difest of the chart.
	digest, err := provenance.DigestFile(fileName)
	if err != nil {
		return fmt.Errorf("failed to generate digest for chart %s: %s", chart.Metadata.Name, err.Error())
	}

	// Add the helm chart to the indexfile.
	err = indexFile.MustAdd(chart.Metadata, fmt.Sprintf("oci://%s/%s:%s", o.registry, o.repository, o.tag), "", digest)
	if err != nil {
		return fmt.Errorf("failed to add entry %s to indexfile: %s", chart.Metadata.Name, err.Error())
	}

	// Delete the file
	err = os.Remove(fileName)
	if err != nil {
		return fmt.Errorf("unable to delete the file %s:%s", fileName, err.Error())
	}

	logrus.Debugf("Added chart %s to index", chart.Metadata.Name)
	return nil
}

// IsOrasRepository checks if the repository is actually a helm chart
// or not. The check is done by finding tags and if we find tags then it
// is a valid repository.
func (o *OCIClient) IsOrasRepository(credentialSecret *corev1.Secret) (bool, error) {

	count := 0

	if o.repository != "" {
		repository, err := o.getOrasRepository(credentialSecret)
		if err != nil {
			return false, err
		}

		// Loop over tags function
		tagsFunc := func(tags []string) error {
			count = len(tags)
			return nil
		}

		err = repository.Tags(context.Background(), "", tagsFunc)
		if err != nil {
			return false, err
		}
	}

	return count != 0, nil
}
