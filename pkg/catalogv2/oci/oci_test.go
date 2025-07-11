package oci

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
	indexFile4 := repo.NewIndexFile()
	indexFile4.Entries["anotherchart"] = repo.ChartVersions{
		&repo.ChartVersion{
			Metadata: &chart.Metadata{
				Name:    "anotherchart",
				Version: "1.0.0",
			},
			Digest: "digest",
		},
	}
	indexFile4.Entries["anotherchartagain"] = repo.ChartVersions{
		&repo.ChartVersion{
			Metadata: &chart.Metadata{
				Name:    "anotherchartagain",
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
		numberOfCharts  *int
		secret          *corev1.Secret
		url             string
		urlPath         string
	}{
		{
			"Can add a specific chart to indexFile if tag is provided",
			repo.NewIndexFile(),
			"",
			&one,
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
			&one,
			nil,
			"",
			"testingchart",
		},
		{
			"Can add charts to index file if registry is provided",
			repo.NewIndexFile(),
			"",
			&two,
			&one,
			nil,
			"",
			"",
		},
		{
			"Should not duplicate charts on indexFile",
			indexFile,
			"",
			&one,
			&one,
			nil,
			"",
			"testingchart:0.1.0",
		},
		{
			"Index file should not have versions that aren't present in the response of /tags/list",
			indexFile3,
			"",
			&two,
			&one,
			nil,
			"",
			"",
		},
		{
			"Index file should not have repositories that aren't present in the response of /_catalog",
			indexFile4,
			"",
			&two,
			&one,
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
			ociClient, err := NewClient(u, repoSpec, nil)
			assert.Nil(t, err)
			i, err := GenerateIndex(ociClient, u, nil, repoSpec, v1.RepoStatus{}, tt.indexFile)
			if tt.expectedErrMsg != "" {
				assert.Contains(t, err.Error(), tt.expectedErrMsg, "wrong error")
			}
			if tt.numberOfCharts != nil {
				assert.Equal(t, len(i.Entries), *tt.numberOfCharts, "number of charts don't match")
			}
			if tt.numberOfEntries != nil {
				assert.Equal(t, len(i.Entries["testingchart"]), *tt.numberOfEntries, "number of entries don't match")
				i.SortEntries()
				assert.NotEmpty(t, i.Entries["testingchart"][0].Digest, "wrong digest for the first entry")
				if *tt.numberOfEntries > 1 {
					assert.Empty(t, i.Entries["testingchart"][1].Digest, "wrong digest for the second entry")
				}
			}
		})
	}
}

func TestGenerateIndexExactAndSubMatch(t *testing.T) {
	base := "forgejo-helm/forgejo"
	catalog := []string{
		"forgejo-helm/forgejo",          // exact match
		"forgejo-helm/forgejo/subchart", // sub-repo match
		"other-repo/chart",
	}

	// HTTP handler to serve catalog and tag-list
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/_catalog":
			// Return the catalog entries
			fmt.Fprintf(w, `{"repositories": ["%s", "%s", "%s"]}`,
				catalog[0], catalog[1], catalog[2])

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, fmt.Sprintf("/v2/%s/tags/list", base)):
			// Return an empty tag list to avoid further branches
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"tags": []}`))

		default:
			// Any other endpoints return 404
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Construct an OCI URL pointing at our test server and base path
	trimmedURL := strings.TrimPrefix(srv.URL, "http://")
	ociURL := fmt.Sprintf("oci://%s/%s", trimmedURL, base)

	// Create a client using insecure HTTP
	client, err := NewClient(ociURL,
		v1.RepoSpec{InsecurePlainHTTP: true, InsecureSkipTLSverify: true},
		(*corev1.Secret)(nil),
	)
	assert.NoError(t, err, "failed to create OCI client")

	indexFile := repo.NewIndexFile()

	// Ensure GenerateIndex neither panics nor returns an unexpected panic
	assert.NotPanics(t, func() {
		_, _ = GenerateIndex(
			client,
			ociURL,
			nil,
			v1.RepoSpec{InsecurePlainHTTP: true, InsecureSkipTLSverify: true},
			v1.RepoStatus{},
			indexFile,
		)
	}, "GenerateIndex should not panic on exact+sub-repo catalog entries")
}

func TestGenerateIndexEmptyRepoExactAndSubMatch(t *testing.T) {
	base := ""
	catalog := []string{
		"forgejo-helm/forgejo",          // exact match
		"forgejo-helm/forgejo/subchart", // sub-repo match
		"other-repo/chart",
	}

	// HTTP handler to serve catalog and tag-list
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v2/_catalog":
			// Return the catalog entries
			fmt.Fprintf(w, `{"repositories": ["%s", "%s", "%s"]}`,
				catalog[0], catalog[1], catalog[2])

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, fmt.Sprintf("/v2/%s/tags/list", base)):
			// Return an empty tag list to avoid further branches
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"tags": []}`))

		default:
			// Any other endpoints return 404
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Construct an OCI URL pointing at our test server and base path
	trimmedURL := strings.TrimPrefix(srv.URL, "http://")
	ociURL := fmt.Sprintf("oci://%s/%s", trimmedURL, base)

	// Create a client using insecure HTTP
	client, err := NewClient(ociURL,
		v1.RepoSpec{InsecurePlainHTTP: true, InsecureSkipTLSverify: true},
		(*corev1.Secret)(nil),
	)
	assert.NoError(t, err, "failed to create OCI client")

	indexFile := repo.NewIndexFile()

	// Ensure the same behaviour when base is empty
	assert.NotPanics(t, func() {
		_, _ = GenerateIndex(
			client,
			ociURL,
			nil,
			v1.RepoSpec{InsecurePlainHTTP: true, InsecureSkipTLSverify: true},
			v1.RepoStatus{},
			indexFile,
		)
	}, "GenerateIndex should not panic on exact+sub-repo catalog entries when the base is empty")
}
