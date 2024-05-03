package oci

import (
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
)

func TestAddtoHelmRepoIndex(t *testing.T) {
	indexFile := repo.NewIndexFile()
	indexFile.Entries["testingchart"] = repo.ChartVersions{
		&repo.ChartVersion{
			Metadata: &chart.Metadata{
				Name:    "testingchart",
				Version: "0.1.0",
			},
			Digest: "digest",
		},
	}

	indexFile2 := repo.NewIndexFile()
	indexFile2.Entries["testingchart"] = repo.ChartVersions{
		&repo.ChartVersion{
			Metadata: &chart.Metadata{
				Name:    "testingchart",
				Version: "0.1.0",
			},
		},
	}

	tests := []struct {
		name                 string
		indexFile            *repo.IndexFile
		expectedErrMsg       string
		maxHelmRepoIndexSize int
	}{
		{
			"returns an error if indexFile size exceeds max size",
			repo.NewIndexFile(),
			"there are a lot of charts inside this oci",
			30,
		},
		{
			"adds the oci artifact to the helm repo index properly without deplication",
			indexFile2,
			"",
			30 * 1024 * 1024, // 30 MiB
		},
		{
			"avoids adding the oci artifact to the helm repo index if it already exists",
			indexFile,
			"",
			30 * 1024 * 1024, // 30 MiB
		},
		{
			"adds the oci artifact to the helm repo index properly",
			repo.NewIndexFile(),
			"",
			30 * 1024 * 1024, // 30 MiB
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := spinRegistry(0, true, true, tt.name, t)
			defer ts.Close()
			ociClient, err := NewClient(fmt.Sprintf("%s/testingchart:0.1.0", strings.Replace(ts.URL, "http", "oci", 1)), v1.RepoSpec{}, nil)
			assert.NoError(t, err)
			orasRepository, err := ociClient.GetOrasRepository()
			orasRepository.PlainHTTP = true
			assert.NoError(t, err)

			maxHelmRepoIndexSize = tt.maxHelmRepoIndexSize
			err = addToHelmRepoIndex(*ociClient, tt.indexFile, orasRepository)
			if tt.expectedErrMsg != "" {
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)
				if len(tt.indexFile.Entries) > 0 {
					assert.Equal(t, len(tt.indexFile.Entries["testingchart"]), 1)
				}
			}
		})
	}
}

func TestGenerateIndex(t *testing.T) {
	indexFile := repo.NewIndexFile()
	indexFile.Entries["testingchart"] = repo.ChartVersions{
		&repo.ChartVersion{
			Metadata: &chart.Metadata{
				Name:    "testingchart",
				Version: "0.1.0",
			},
			Digest: "digest",
		},
	}

	indexFile2 := repo.NewIndexFile()
	indexFile2.Entries["testingchart"] = repo.ChartVersions{
		&repo.ChartVersion{
			Metadata: &chart.Metadata{
				Name:    "testingchart",
				Version: "0.1.0",
			},
		},
	}

	indexFile3 := repo.NewIndexFile()
	indexFile3.Entries["testingchart"] = repo.ChartVersions{
		&repo.ChartVersion{
			Metadata: &chart.Metadata{
				Name:    "testingchart",
				Version: "1.0.0",
			},
			Digest: "digest",
		},
	}
	one := 1
	two := 2

	tests := []struct {
		name            string
		indexFile       *repo.IndexFile
		expectedErrMsg  string
		numberOfEntries *int
		secret          *corev1.Secret
		url             string
		urlPath         string
	}{
		{
			"returns an error if url is invalid",
			repo.NewIndexFile(),
			"failed to create an OCI client for url",
			nil,
			nil,
			"invalidUrl//",
			"",
		},
		{
			"returns an error if url is not an oras repository",
			repo.NewIndexFile(),
			"failed to create an OCI client for url",
			nil,
			nil,
			"http://github.com/rancher/charts",
			"",
		},

		{
			"Can add a specific chart to indexFile if tag is provided",
			repo.NewIndexFile(),
			"",
			&one,
			nil,
			"",
			"testingchart:0.1.0",
		},
		{
			"Can add charts to index file if repository is provided",
			repo.NewIndexFile(),
			"",
			&two,
			nil,
			"",
			"testingchart",
		},
		{
			"Can add charts to index file if registry is provided",
			repo.NewIndexFile(),
			"",
			&two,
			nil,
			"",
			"",
		},
		{
			"Should not duplicate charts on indexFile",
			indexFile,
			"",
			&one,
			nil,
			"",
			"testingchart:0.1.0",
		},
		{
			"Index file should not have versions that aren't present in the respose of /tags/list",
			indexFile3,
			"",
			&two,
			nil,
			"",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := spinRegistry(0, true, true, tt.name, t)
			defer ts.Close()
			u := fmt.Sprintf("%s/%s", strings.Replace(ts.URL, "http", "oci", 1), tt.urlPath)
			if tt.url != "" {
				u = tt.url
			}
			repoSpec := v1.RepoSpec{InsecurePlainHTTP: true, InsecureSkipTLSverify: true}
			i, err := GenerateIndex(u, nil, repoSpec, v1.RepoStatus{}, tt.indexFile)
			if tt.expectedErrMsg != "" {
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			}
			if tt.numberOfEntries != nil {
				assert.Equal(t, len(i.Entries["testingchart"]), *tt.numberOfEntries)
				i.SortEntries()
				assert.NotEmpty(t, i.Entries["testingchart"][0].Digest)
				if *tt.numberOfEntries > 1 {
					assert.Empty(t, i.Entries["testingchart"][1].Digest)
				}
			}
		})
	}
}
