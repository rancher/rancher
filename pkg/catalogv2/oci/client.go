package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	ocispecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/oci/capturewindowclient"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chart/loader"
	helmregistry "helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/helm/pkg/provenance"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

// maxHelmChartTar defines what is the max size of helm chart we support.
const maxHelmChartTarSize int64 = 20 * 1024 * 1024 // 20 MiB

// Client is an OCI client that manages Helm charts in OCI based Helm registries.
type Client struct {
	// URL refers to the OCI url provided by the user ie. dp.apps.rancher.io/charts/etcd:1.0.2
	URL string
	// registry is the registry part of the URL ie. dp.apps.rancher.io
	registry string
	// repository is the repository part of the URL ie. charts/etcd
	repository string
	// tag is the tag part of the URL ie. 1.0.2
	tag string

	HTTPClient               http.Client
	insecure                 bool
	caBundle                 []byte
	insecurePlainHTTP        bool
	exponentialBackOffValues *catalogv1.ExponentialBackOffValues

	username string
	password string
}

// NewClient returns a new Client along with parsing
// the URL provided and fetching the credentials.
func NewClient(url string, clusterRepoSpec catalogv1.RepoSpec, credentialSecret *v1.Secret) (*Client, error) {
	ociClient := &Client{
		URL:                      url,
		insecure:                 clusterRepoSpec.InsecureSkipTLSverify,
		caBundle:                 clusterRepoSpec.CABundle,
		insecurePlainHTTP:        clusterRepoSpec.InsecurePlainHTTP,
		exponentialBackOffValues: clusterRepoSpec.ExponentialBackOffValues,
	}

	err := ociClient.parseURL()
	if err != nil {
		return nil, err
	}

	if credentialSecret != nil {
		if credentialSecret.Type != v1.SecretTypeBasicAuth {
			return nil, fmt.Errorf("only basic auth credential is supported")
		}

		username, password := string(credentialSecret.Data[v1.BasicAuthUsernameKey]), string(credentialSecret.Data[v1.BasicAuthPasswordKey])
		if len(password) == 0 || len(username) == 0 {
			return nil, fmt.Errorf("username or password is empty")
		}
		ociClient.username = username
		ociClient.password = password
	}

	err = ociClient.SetAuthClient()
	if err != nil {
		return nil, err
	}

	return ociClient, nil
}

// parseURL parses the provided OCI URL into sub
// parts such as registry, repository and tag.
func (o *Client) parseURL() error {
	// Remove any slash at the end of the URL
	url := strings.TrimSuffix(o.URL, "/")

	// Remove the oci scheme from the start of the URL
	url = strings.TrimPrefix(url, "oci://")

	// If the URL contains a slash, then it must have a repository and/or tag
	if strings.Contains(url, "/") {
		var ref registry.Reference
		ref, err := registry.ParseReference(url)
		if err != nil {
			return fmt.Errorf("failed to parse OCI URL '%s' value: %w", o.URL, err)
		}

		o.registry = ref.Registry
		o.repository = ref.Repository
		o.tag = ref.Reference

		// If the URL doesn't contain any slash, then it must have only the registry part of it.
	} else {
		o.registry = url
	}

	return nil
}

// fetchChart fetchs the chart specified by the oras repository. It first downloads it into the
// oras memory and then saves it into a file and returns the file path.
func (o *Client) fetchChart(orasRepository *remote.Repository) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ociURL := fmt.Sprintf("%s/%s:%s", o.registry, o.repository, o.tag)

	// Create an oras memory to copy the oci artifact into.
	memoryStore := memory.New()
	manifest, err := oras.Copy(ctx, orasRepository, o.tag, memoryStore, "", oras.CopyOptions{
		CopyGraphOptions: oras.CopyGraphOptions{
			PreCopy: func(ctx context.Context, desc ocispecv1.Descriptor) error {
				// Download only helm chart related descriptors.
				if desc.MediaType == ocispecv1.MediaTypeImageManifest ||
					desc.MediaType == helmregistry.ChartLayerMediaType {
					// We cannot load huge amounts of data into the memory
					// and so we are defining a limit before fetching.
					if desc.Size > maxHelmChartTarSize {
						return fmt.Errorf("the oci artifact %s:%s has size more than %d which is not supported", o.repository, o.tag, maxHelmChartTarSize)
					}
					return nil
				}

				return oras.SkipNode
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("unable to oras copy the remote oci artifact %s: %w", ociURL, err)
	}

	// Helm codebase sets oci artifacts manifest mediatype as ocispecv1.MediaTypeImageManifest
	// https://github.com/oras-project/oras-go/blob/v1/pkg/content/manifest.go#L89C22-L89C44 referenced by helm codebase
	if manifest.MediaType != ocispecv1.MediaTypeImageManifest {
		return "", fmt.Errorf("the oci artifact %s is not a helm chart. The OCI URL must contain only helm charts", ociURL)
	}

	// Fetch the manifest blob of the oci artifact
	manifestblob, err := content.FetchAll(ctx, memoryStore, manifest)
	if err != nil {
		return "", fmt.Errorf("unable to fetch the manifest blob of %s: %w", ociURL, err)
	}
	var manifestJSON ocispecv1.Manifest
	err = json.Unmarshal(manifestblob, &manifestJSON)
	if err != nil {
		return "", fmt.Errorf("unable to unmarshal manifest blob of %s: %w", ociURL, err)
	}

	// Create a temp file to store the helm chart tar
	tempFile, err := os.CreateTemp("", "helm-")
	if err != nil {
		return "", fmt.Errorf("unable to create temp file for storing the oci artifact")
	}

	// Checking if the OCI artifact is of type helm config ?
	if manifestJSON.ArtifactType == helmregistry.ConfigMediaType || manifestJSON.Config.MediaType == helmregistry.ConfigMediaType {
		// find the layer of helm chart type and fetch it
		for _, layer := range manifestJSON.Layers {
			if layer.MediaType == helmregistry.ChartLayerMediaType {
				chartTar, err := content.FetchAll(ctx, memoryStore, layer)
				if err != nil {
					return "", fmt.Errorf("unable to fetch chart blob of %s: %w", ociURL, err)
				}

				err = os.WriteFile(tempFile.Name(), chartTar, 0o600)
				if err != nil {
					return "", fmt.Errorf("unable to write chart %s into file %s: %w", ociURL, tempFile.Name(), err)
				}

				return tempFile.Name(), nil
			}
		}
	}

	return tempFile.Name(), fmt.Errorf("the oci artifact %s is not a helm chart. The OCI URL must contain only helm charts", ociURL)
}

// getAuthClient creates an oras auth client that can be used
// in creating an oras registry client or oras repository client.
func (o *Client) SetAuthClient() error {
	config := &tls.Config{
		InsecureSkipVerify: o.insecure,
	}
	if len(o.caBundle) > 0 {
		cert, err := x509.ParseCertificate(o.caBundle)
		if err != nil {
			return err
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			logrus.Debugf("getting system cert pool failed: %v", err)
			pool = x509.NewCertPool()
		}
		pool.AddCert(cert)

		config.RootCAs = pool
	}
	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	baseTransport.TLSClientConfig = config

	o.HTTPClient = http.Client{
		Transport: capturewindowclient.NewTransport(baseTransport),
	}

	return nil
}

// GetOrasRegistry returns the oras registry client along with
// setting credentials to authenticate with the registry.
func (o *Client) GetOrasRegistry() (*remote.Registry, error) {
	orasRegistry, err := remote.NewRegistry(o.registry)
	if err != nil {
		return nil, err
	}
	orasRegistry.PlainHTTP = o.insecurePlainHTTP

	orasRegistry.Client = &auth.Client{
		Credential: func(ctx context.Context, reg string) (auth.Credential, error) {
			return auth.Credential{
				Username: o.username,
				Password: o.password,
			}, nil
		},
		Client: &o.HTTPClient,
	}

	return orasRegistry, nil
}

// GetOrasRepository returns the oras repository client along with
// setting credentials to authenticate with the registry/repository.
func (o *Client) GetOrasRepository() (*remote.Repository, error) {
	orasRepository, err := remote.NewRepository(fmt.Sprintf("%s/%s", o.registry, o.repository))
	if err != nil {
		return nil, err
	}
	orasRepository.PlainHTTP = o.insecurePlainHTTP

	orasRepository.Client = &auth.Client{
		Credential: func(ctx context.Context, reg string) (auth.Credential, error) {
			return auth.Credential{
				Username: o.username,
				Password: o.password,
			}, nil
		},
		Client: &o.HTTPClient,
	}

	return orasRepository, nil
}

// addToIndex adds the given helm chart entry into the helm repo index provided.
func (o *Client) addToIndex(indexFile *repo.IndexFile, chartTarFilePath string) error {
	// Load the Chart into chart golang struct.
	chart, err := loader.Load(chartTarFilePath)
	if err != nil {
		return fmt.Errorf("failed to load the chart %s: %w", chartTarFilePath, err)
	}

	// Generate the digest of the chart.
	digest, err := provenance.DigestFile(chartTarFilePath)
	if err != nil {
		return fmt.Errorf("failed to generate digest for chart %s: %w", chart.Metadata.Name, err)
	}

	// Add the helm chart to the indexfile.
	err = indexFile.MustAdd(chart.Metadata, fmt.Sprintf("oci://%s/%s:%s", o.registry, o.repository, o.tag), "", digest)
	if err != nil {
		return fmt.Errorf("failed to add entry %s to indexfile: %w", chart.Metadata.Name, err)
	}

	// For OCI repositories, the created date is not exposed and so Helm library defaults to time.Now()
	// This is misleading and so emptying the created date field
	indexFile.Entries[chart.Metadata.Name][len(indexFile.Entries[chart.Metadata.Name])-1].Created = time.Time{}

	logrus.Debugf("Added chart %s %s to index", chart.Metadata.Name, chart.Metadata.Version)
	return nil
}

// IsOrasRepository checks if the repository is actually an oci artifact or not.
// The check is done by finding tags and if we find tags then it is valid repo.
func (o *Client) IsOrasRepository() (bool, error) {
	count := 0
	ociRepo := fmt.Sprintf("%s/%s", o.registry, o.repository)

	if o.repository != "" {
		repository, err := o.GetOrasRepository()
		if err != nil {
			return false, fmt.Errorf("failed to create an oras repository for %s: %w", ociRepo, err)
		}

		// Loop over tags function
		tagsFunc := func(tags []string) error {
			count = len(tags)
			return nil
		}

		err = repository.Tags(context.Background(), "", tagsFunc)
		if err != nil {
			if IsErrorCode(err, errcode.ErrorCodeNameUnknown) ||
				IsErrorMessage(err, "invalid repository name") {
				return false, nil
			}
			return false, err
		}
	}

	return count != 0, nil
}

// IsErrorCode returns true if err is an Error and its Code equals to code.
func IsErrorCode(err error, code string) bool {
	var ec errcode.Error
	return errors.As(err, &ec) && ec.Code == code
}

// IsErrorMessage returns true if err is an Error and its message is found.
func IsErrorMessage(err error, message string) bool {
	var ec errcode.Error
	return errors.As(err, &ec) && strings.Contains(ec.Message, message)
}
