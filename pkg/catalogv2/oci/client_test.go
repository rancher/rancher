package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
)

func TestNewClient(t *testing.T) {
	testCases := []struct {
		name               string
		url                string
		repoSpec           v1.RepoSpec
		secret             *corev1.Secret
		expectedRegistry   string
		expectedRepository string
		expectedTag        string
		expectedErr        error
	}{
		{
			name:               "works with full OCI URL with tag",
			url:                "oci://example.com/charts/etcd:0.1.1",
			repoSpec:           v1.RepoSpec{},
			expectedRegistry:   "example.com",
			expectedRepository: "charts/etcd",
			expectedTag:        "0.1.1",
			expectedErr:        nil,
		},
		{
			name:               "works with OCI URL with no tag and two word namespace",
			url:                "oci://example.com/charts/etcd",
			repoSpec:           v1.RepoSpec{},
			expectedRegistry:   "example.com",
			expectedRepository: "charts/etcd",
			expectedTag:        "",
			expectedErr:        nil,
		},
		{
			name:               "works with OCI URL with no tag and only single word namespace",
			url:                "oci://example.com/charts",
			repoSpec:           v1.RepoSpec{},
			expectedRegistry:   "example.com",
			expectedRepository: "charts",
			expectedTag:        "",
			expectedErr:        nil,
		},
		{
			name:               "works with OCI URL with only registry",
			url:                "oci://example.com",
			repoSpec:           v1.RepoSpec{},
			expectedRegistry:   "example.com",
			expectedRepository: "",
			expectedTag:        "",
			expectedErr:        nil,
		},
		{
			name:        "fails with invalid OCI prefix",
			url:         "oc://example.com",
			repoSpec:    v1.RepoSpec{},
			expectedErr: errors.New("invalid reference"),
		},
		{
			name:        "fails with invalid OCI URL with double slashes",
			url:         "oc://example.com//",
			repoSpec:    v1.RepoSpec{},
			expectedErr: errors.New("invalid reference"),
		},
		{
			name:        "fails with invalid OCI URL with unknown characters",
			url:         "oc://example.com*)//",
			repoSpec:    v1.RepoSpec{},
			expectedErr: errors.New("invalid reference"),
		},
		{
			name:     "works fine with basic auth",
			repoSpec: v1.RepoSpec{},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretname",
				},
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("test"),
					corev1.BasicAuthPasswordKey: []byte("test"),
				},
				Type: corev1.SecretTypeBasicAuth,
			},
			expectedErr: nil,
		},
		{
			name:     "doesn't work fine with invalid auth",
			repoSpec: v1.RepoSpec{},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretname",
				},
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("test"),
				},
				Type: corev1.SecretTypeBasicAuth,
			},
			expectedErr: errors.New("username or password is empty"),
		},
		{
			name: "doesn't work with invalid secret type",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secretname",
				},
				Data: map[string][]byte{
					corev1.BasicAuthUsernameKey: []byte("test"),
				},
			},
			expectedErr: errors.New("only basic auth credential is supported"),
		},
		{
			name: "sets insecure with insecure mentioned",
			repoSpec: v1.RepoSpec{
				InsecureSkipTLSverify: true,
			},
			secret:      nil,
			expectedErr: nil,
		},
		{
			name:        "sets insecurePlainHTTP with insecurePlainHTTP set as true",
			secret:      nil,
			expectedErr: nil,
			repoSpec: v1.RepoSpec{
				InsecurePlainHTTP: true,
			},
		},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		ociClient, err := NewClient(tc.url, tc.repoSpec, tc.secret)

		if tc.expectedErr != nil {
			assert.ErrorContains(err, tc.expectedErr.Error())
		} else {
			assert.NoError(err)

			assert.Equal(tc.expectedRegistry, ociClient.registry)
			assert.Equal(tc.expectedRepository, ociClient.repository)
			assert.Equal(tc.expectedTag, ociClient.tag)
			assert.Equal(tc.repoSpec.InsecureSkipTLSverify, ociClient.insecure)
			assert.Equal(tc.repoSpec.InsecurePlainHTTP, ociClient.insecurePlainHTTP)

			if tc.secret != nil {
				assert.Equal(ociClient.username, "test")
				assert.Equal(ociClient.password, "test")
			}
		}
	}
}

func TestFetchChart(t *testing.T) {
	type testcase struct {
		name          string
		expectedErr   string
		expectedFound bool
		expectedFile  bool
	}

	testCase1 := testcase{
		name:          "fetching a chart works fine with a tag",
		expectedErr:   "",
		expectedFound: true,
		expectedFile:  true,
	}

	testCase2 := testcase{
		name:          "fetches no chart since manifest config media type is not Helm Media type",
		expectedErr:   "",
		expectedFound: false,
		expectedFile:  false,
	}

	testCase3 := testcase{
		name:          "fetches no chart since chart layer is not Helm Chart type",
		expectedErr:   "",
		expectedFound: false,
		expectedFile:  false,
	}

	testCases := []testcase{
		testCase1,
		testCase2,
		testCase3,
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		// Create an OCI Helm Chart Image
		configBlob := []byte("config")
		configDesc := ocispec.Descriptor{
			MediaType: registry.ConfigMediaType,
			Digest:    digest.FromBytes(configBlob),
			Size:      int64(len(configBlob)),
		}
		layerBlob := []byte("layer")
		layerDesc := ocispec.Descriptor{
			MediaType: registry.ChartLayerMediaType,
			Digest:    digest.FromBytes(layerBlob),
			Size:      int64(len(layerBlob)),
		}

		// Modify test data according to the testcase
		switch tc.name {
		case "fetches no chart since manifest config media type is not Helm Media type":
			configDesc.MediaType = ocispec.MediaTypeImageConfig

		case "fetches no chart since chart layer is not Helm Chart type":
			layerDesc.MediaType = ocispec.MediaTypeImageLayer
		}

		manifest := ocispec.Manifest{
			MediaType: ocispec.MediaTypeImageManifest,
			Config:    configDesc,
			Layers:    []ocispec.Descriptor{layerDesc},
		}
		manifestJSON, err := json.Marshal(manifest)
		assert.NoError(err)

		manifestDesc := ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageManifest,
			Digest:    digest.FromBytes(manifestJSON),
			Size:      int64(len(manifestJSON)),
		}

		// Create a OCI Registry Server
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			switch r.URL.Path {
			case "/v2/test/blobs/" + layerDesc.Digest.String():
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Docker-Content-Digest", layerDesc.Digest.String())
				if _, err := w.Write(layerBlob); err != nil {
					assert.NoError(err)
				}
			case "/v2/test/blobs/" + configDesc.Digest.String():
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Docker-Content-Digest", configDesc.Digest.String())
				if _, err := w.Write(configBlob); err != nil {
					assert.NoError(err)
				}
			case "/v2/test/manifests/v1.0.2":
				if accept := r.Header.Get("Accept"); !strings.Contains(accept, manifestDesc.MediaType) {
					assert.NoError(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", manifestDesc.MediaType)
				w.Header().Set("Docker-Content-Digest", manifestDesc.Digest.String())
				if _, err := w.Write(manifestJSON); err != nil {
					assert.NoError(err)
				}
			}
		}))
		defer ts.Close()

		ociClient, err := NewClient(fmt.Sprintf("%s/test:v1.0.2", strings.Replace(ts.URL, "http", "oci", 1)), v1.RepoSpec{}, nil)
		assert.NoError(err)

		orasReposistory, err := ociClient.GetOrasRepository()
		orasReposistory.PlainHTTP = true
		assert.NoError(err)

		_, found, err := ociClient.fetchChart(orasReposistory)
		if tc.expectedErr == "" {
			assert.NoError(err)
		} else {
			assert.Contains(err.Error(), tc.expectedErr)
		}
		assert.Equal(found, tc.expectedFound)
	}
}

func TestGetOrasRegistry(t *testing.T) {
	testCases := []struct {
		name              string
		expectedErr       error
		insecurePlainHTTP bool
	}{
		{
			name:              "fetching oras registry works fine without auth",
			expectedErr:       nil,
			insecurePlainHTTP: false,
		},
		{
			name:              "fetching oras repository works fine with plainHTTP",
			expectedErr:       nil,
			insecurePlainHTTP: true,
		},
	}

	for _, tc := range testCases {
		ociClient, err := NewClient("oci://example.com/charts/test:1.2.2", v1.RepoSpec{InsecurePlainHTTP: tc.insecurePlainHTTP}, nil)
		assert.NoError(t, err)

		orasRegistry, err := ociClient.GetOrasRegistry()
		assert.Equal(t, orasRegistry.PlainHTTP, tc.insecurePlainHTTP)

		if tc.expectedErr != nil {
			assert.ErrorContains(t, err, tc.expectedErr.Error())
		} else {
			assert.NoError(t, err)
		}
	}
}

func TestGetOrasRepository(t *testing.T) {
	testCases := []struct {
		name              string
		expectedErr       error
		insecurePlainHTTP bool
	}{
		{
			name:              "fetching oras registry works fine without auth",
			expectedErr:       nil,
			insecurePlainHTTP: false,
		},
		{
			name:              "fetching oras repository works fine with plainHTTP",
			expectedErr:       nil,
			insecurePlainHTTP: true,
		},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		ociClient, err := NewClient("oci://example.com/charts/test:1.2.2", v1.RepoSpec{InsecurePlainHTTP: tc.insecurePlainHTTP}, nil)
		assert.NoError(err)

		orasRepo, err := ociClient.GetOrasRepository()
		assert.Equal(orasRepo.PlainHTTP, tc.insecurePlainHTTP)
		if tc.expectedErr != nil {
			assert.ErrorContains(err, tc.expectedErr.Error())
		} else {
			assert.NoError(err)
		}
	}
}

func TestAddToIndex(t *testing.T) {
	indexFile2 := repo.NewIndexFile()
	indexFile2.Entries = nil

	testCases := []struct {
		name        string
		chartName   string
		fileName    string
		indexFile   *repo.IndexFile
		expectedErr error
	}{
		{
			name:        "adding a chart to index works fine",
			chartName:   "testingchart",
			fileName:    "../../../tests/testdata/testingchart-0.1.0.tgz",
			indexFile:   repo.NewIndexFile(),
			expectedErr: nil,
		},
		{
			name:        "adding a chart to index doesn't work since file not found",
			chartName:   "testingchart",
			fileName:    "",
			indexFile:   repo.NewIndexFile(),
			expectedErr: errors.New("failed to load the chart"),
		},
		{
			name:        "adding a chart to index doesn't work since adding to index fails",
			chartName:   "testingchart",
			fileName:    "../../../tests/testdata/testingchart-0.1.0.tgz",
			indexFile:   indexFile2,
			expectedErr: errors.New("failed to add entry"),
		},
	}
	assert := assert.New(t)
	for _, tc := range testCases {

		ociClient, err := NewClient("oci://example.com/testingchart:0.1.0", v1.RepoSpec{}, nil)
		assert.NoError(err)

		err = ociClient.addToIndex(tc.indexFile, tc.chartName, tc.fileName)
		if tc.expectedErr != nil {
			assert.ErrorContains(err, tc.expectedErr.Error())
		} else {
			assert.NoError(err)
		}
	}
}

func TestIsOrasRepository(t *testing.T) {
	indexFile2 := repo.NewIndexFile()
	indexFile2.Entries = nil

	type Tags struct {
		Tags []string `json:"tags"`
	}

	testTagList := Tags{
		Tags: []string{"tag"},
	}

	testCases := []struct {
		name        string
		URL         string
		ok          bool
		tags        Tags
		expectedErr error
	}{
		{
			name:        "it is a oras repository",
			URL:         "oci://example.com/testingchart",
			ok:          true,
			tags:        testTagList,
			expectedErr: nil,
		},
		{
			name:        "it is not a oras repository since no tags are present",
			URL:         "oci://example.com/testingchart",
			ok:          false,
			tags:        Tags{},
			expectedErr: nil,
		},
	}
	assert := assert.New(t)
	for _, tc := range testCases {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := r.URL.Path
			m := r.Method
			switch {
			case p == "/v2/" && m == "GET":
				w.WriteHeader(http.StatusOK)
			case p == "/v2/testingchart/tags/list" && m == "GET":
				if err := json.NewEncoder(w).Encode(tc.tags); err != nil {
					http.Error(w, "error encoding", http.StatusBadRequest)
				}
			}
		}))
		defer ts.Close()

		ociClient, err := NewClient(fmt.Sprintf("%s/testingchart", strings.Replace(ts.URL, "http", "oci", 1)), v1.RepoSpec{InsecurePlainHTTP: true}, nil)
		assert.NoError(err)

		ok, err := ociClient.IsOrasRepository()
		if tc.expectedErr != nil {
			assert.ErrorContains(err, tc.expectedErr.Error())
		} else {
			assert.NoError(err)
		}
		assert.Equal(ok, tc.ok)
	}
}

func TestInsecure(t *testing.T) {
	testCases := []struct {
		name        string
		URL         string
		insecure    bool
		expectedErr error
	}{
		{
			name:        "fails when insecure is not specified",
			URL:         "oci://example.com/testingchart",
			expectedErr: errors.New("failed to verify certificate"),
		},
		{
			name:        "passes when insecure is specified",
			URL:         "oci://example.com/testingchart",
			insecure:    true,
			expectedErr: nil,
		},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := r.URL.Path
			m := r.Method
			switch {
			case p == "/v2/" && m == "GET":
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer ts.Close()

		ociClient, err := NewClient(fmt.Sprintf("%s/testingchart", strings.Replace(ts.URL, "https", "oci", 1)),
			v1.RepoSpec{
				InsecureSkipTLSverify: tc.insecure,
			},
			nil)
		assert.NoError(err)

		orasRegistry, err := ociClient.GetOrasRegistry()
		assert.NoError(err)

		err = orasRegistry.Ping(context.Background())
		if tc.expectedErr != nil {
			assert.ErrorContains(err, tc.expectedErr.Error())
		} else {
			assert.NoError(err)
		}
	}
}

func TestCaBundle(t *testing.T) {
	testCases := []struct {
		name        string
		caBundle    bool
		expectedErr error
	}{
		{
			name:        "fails when caBundle is not specified",
			expectedErr: errors.New("failed to verify certificate"),
		},
		{
			name:        "passes when caBundle is specified",
			caBundle:    true,
			expectedErr: nil,
		},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer ts.Close()

		var repoSpec v1.RepoSpec
		if tc.caBundle {
			repoSpec = v1.RepoSpec{
				CABundle: ts.Certificate().Raw,
			}
		}

		ociClient, err := NewClient(fmt.Sprintf("%s/testingchart", strings.Replace(ts.URL, "https", "oci", 1)),
			repoSpec,
			nil)
		assert.NoError(err)

		orasRegistry, err := ociClient.GetOrasRegistry()
		assert.NoError(err)

		err = orasRegistry.Ping(context.Background())
		if tc.expectedErr != nil {
			assert.ErrorContains(err, tc.expectedErr.Error())
		} else {
			assert.NoError(err)
		}
	}
}
