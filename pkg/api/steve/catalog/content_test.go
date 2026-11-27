package catalog

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/apis/catalog.cattle.io"
	catalogv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/content"
	"github.com/rancher/rancher/pkg/catalogv2/git"
	"github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
	"github.com/rancher/steve/pkg/server/handler"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/schemas"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	helmrepo "helm.sh/helm/v4/pkg/repo/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func TestSetIconHeaders(t *testing.T) {
	tests := []struct {
		name   string
		suffix string
	}{
		{
			name:   ".svg suffix",
			suffix: ".svg",
		},
		{
			name:   ".png suffix",
			suffix: ".png",
		},
		{
			name:   "no suffix",
			suffix: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			setIconHeaders(rw, tt.suffix)
			resp := rw.Result()
			assert.Equal(t, []string{"max-age=31536000, public"}, resp.Header["Cache-Control"])
			assert.Equal(t, []string{"default-src 'none'; style-src 'unsafe-inline'; sandbox"}, resp.Header["Content-Security-Policy"])
			assert.Equal(t, []string{"nosniff"}, resp.Header["X-Content-Type-Options"])
			if tt.suffix == ".svg" {
				assert.Equal(t, []string{"image/svg+xml"}, resp.Header["Content-Type"])
			} else {
				assert.Nil(t, resp.Header["Content-Type"])
			}
		})
	}
}

func TestWriteErrorRetryAfter(t *testing.T) {
	notReady := apierror.WrapAPIError(git.ErrNotCheckedOut, validation.ClusterUnavailable, git.ErrNotCheckedOut.Error())

	tests := []struct {
		name               string
		err                error
		expectedRetryAfter string
	}{
		{name: "repository not checked out yet", err: notReady, expectedRetryAfter: "5"},
		{name: "other 503", err: apierror.NewAPIError(validation.ClusterUnavailable, "unrelated 503")},
		{name: "other error", err: errors.New("boom")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			var written error
			apiContext := &types.APIRequest{
				ErrorHandler: func(_ *types.APIRequest, err error) { written = err },
			}

			(&contentDownload{}).writeError(apiContext, rw, tt.err)

			assert.Equal(t, tt.err, written, "the error should still be written")
			assert.Equal(t, tt.expectedRetryAfter, rw.Header().Get("Retry-After"))
		})
	}
}

func TestClusterRepoIndexAvailability(t *testing.T) {
	const schemaID = "catalog.cattle.io.clusterrepo"
	const path = "/v1/catalog.cattle.io.clusterrepos/example"
	repo := &catalogv1.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{Name: "example", UID: "repo-uid"},
		Spec: catalogv1.RepoSpec{
			GitRepo:   "https://example.com/charts",
			GitBranch: "main",
		},
	}
	repos := cache.NewIndexer(cache.MetaNamespaceKeyFunc, nil)
	require.NoError(t, repos.Add(repo))
	maps := cache.NewIndexer(cache.MetaNamespaceKeyFunc, nil)
	repoCache := generic.NewNonNamespacedCache[*catalogv1.ClusterRepo](repos, catalogv1.Resource("clusterrepos"))
	manager := content.NewManager(
		&fake.FakeDiscovery{
			Fake:               &clienttesting.Fake{},
			FakedServerVersion: &version.Info{GitVersion: "v1.35.0"},
		},
		generic.NewCache[*corev1.ConfigMap](maps, corev1.Resource("configmaps")),
		nil,
		repoCache,
	)

	factory := &catalogSchemaFactory{schemas: types.EmptyAPISchemas()}
	addSchemas(&steve.Server{BaseSchemas: factory.schemas, SchemaFactory: factory}, nil, &contentDownload{contentManager: manager})
	apiSchema := types.APISchema{
		Schema: &schemas.Schema{
			ID:              schemaID,
			PluralName:      "catalog.cattle.io.clusterrepos",
			ResourceMethods: []string{http.MethodGet},
		},
		Store: clusterRepoStore{},
	}
	for _, template := range factory.templates {
		if template.Group == catalog.GroupName && template.Kind == "ClusterRepo" {
			template.Customize(&apiSchema)
		}
	}
	require.NoError(t, factory.schemas.AddSchema(apiSchema))
	_, server, err := handler.New(&rest.Config{Host: "https://localhost"}, factory, nil, http.NotFoundHandler(), nil, nil)
	require.NoError(t, err)

	get := func(query string) *httptest.ResponseRecorder {
		rw := httptest.NewRecorder()
		server.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "https://rancher.example"+path+query, nil))
		return rw
	}

	t.Run("unpublished_index", func(t *testing.T) {
		rw := get("?link=index")
		assert.Equal(t, http.StatusNotFound, rw.Code, rw.Body.String())
	})
	t.Run("unpublished_chart_content", func(t *testing.T) {
		for _, link := range []string{"chart", "icon", "info"} {
			rw := get("?link=" + link + "&chartName=example-chart&version=1.0.0")
			assert.Equal(t, http.StatusNotFound, rw.Code, link)
		}
	})

	published := repo.DeepCopy()
	published.Status.IndexConfigMapNamespace = "cattle-system"
	published.Status.IndexConfigMapName = "example-index"
	require.NoError(t, repos.Update(published))
	t.Run("missing_referenced_configmap", func(t *testing.T) {
		rw := get("?link=index")
		assert.Equal(t, http.StatusInternalServerError, rw.Code, rw.Body.String())
	})

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       published.Status.IndexConfigMapNamespace,
			Name:            published.Status.IndexConfigMapName,
			ResourceVersion: "1",
			OwnerReferences: []metav1.OwnerReference{{UID: repo.UID}},
		},
		BinaryData: map[string][]byte{"content": []byte("invalid gzip data")},
	}
	require.NoError(t, maps.Add(cm))
	t.Run("corrupt_index", func(t *testing.T) {
		rw := get("?link=index")
		assert.Equal(t, http.StatusInternalServerError, rw.Code, rw.Body.String())
	})

	var data bytes.Buffer
	gz := gzip.NewWriter(&data)
	_, err = gz.Write([]byte(`{"apiVersion":"v1","entries":{"example-chart":[{"apiVersion":"v2","name":"example-chart","version":"1.0.0","urls":["example-chart-1.0.0.tgz"]}]}}`))
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	ready := cm.DeepCopy()
	ready.ResourceVersion = "2"
	ready.BinaryData["content"] = data.Bytes()
	require.NoError(t, maps.Update(ready))
	t.Run("published_index", func(t *testing.T) {
		for range 2 {
			rw := get("?link=index&skipFilter=true")
			require.Equal(t, http.StatusOK, rw.Code, rw.Body.String())
			var index helmrepo.IndexFile
			require.NoError(t, json.NewDecoder(rw.Body).Decode(&index))
			require.Len(t, index.Entries["example-chart"], 1)
			chart := index.Entries["example-chart"][0]
			assert.Equal(t, "1.0.0", chart.Version)
			assert.Equal(t, []string{"https://rancher.example" + path + "?chartName=example-chart&link=chart&version=1.0.0"}, chart.URLs)
		}
	})
}

type catalogSchemaFactory struct {
	schema.Factory
	schemas   *types.APISchemas
	templates []schema.Template
}

func (f *catalogSchemaFactory) Schemas(user.Info) (*types.APISchemas, error) {
	return f.schemas, nil
}

func (f *catalogSchemaFactory) AddTemplate(templates ...schema.Template) {
	f.templates = append(f.templates, templates...)
}

type clusterRepoStore struct {
	types.Store
}

func (clusterRepoStore) ByID(*types.APIRequest, *types.APISchema, string) (types.APIObject, error) {
	return types.APIObject{}, nil
}
