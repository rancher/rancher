package catalog

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/rancher/apiserver/pkg/types"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/helm"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"helm.sh/helm/v3/pkg/repo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type indexDownload struct {
	configMaps   corecontrollers.ConfigMapClient
	repos        catalogcontrollers.RepoClient
	secrets      corecontrollers.SecretClient
	clusterRepos catalogcontrollers.ClusterRepoClient
}

func (i *indexDownload) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	apiContext := types.GetAPIContext(req.Context())
	switch apiContext.Link {
	case "index":
		if err := i.serveIndex(apiContext); err != nil {
			apiContext.WriteError(err)
		}
	case "info":
		if err := i.serveInfo(apiContext); err != nil {
			apiContext.WriteError(err)
		}
	}
}

func (i *indexDownload) serveIndex(apiContext *types.APIRequest) error {
	index, err := i.getIndex(apiContext)
	if err != nil {
		return err
	}

	apiContext.Response.Header().Set("Content-Type", "application/json")
	_, err = apiContext.Response.Write(index)
	return err
}

func (i *indexDownload) serveInfo(apiContext *types.APIRequest) error {
	query := apiContext.Request.URL.Query()
	chartName := query.Get("chartName")
	version := query.Get("version")

	if chartName == "" {
		return validation.NotFound
	}

	index, err := i.getIndex(apiContext)
	if err != nil {
		return err
	}

	indexFile := &repo.IndexFile{}
	if err := json.Unmarshal(index, indexFile); err != nil {
		return err
	}

	release, err := indexFile.Get(chartName, version)
	if err != nil {
		return err
	}

	repo, err := i.getRepo(apiContext)
	if err != nil {
		return err
	}

	client, err := helm.Client(i.secrets, repo.spec, repo.metadata.Namespace)
	if err != nil {
		return err
	}
	defer client.CloseIdleConnections()

	var (
		lastError error
	)
	for _, url := range release.URLs {
		lastError = i.writeInfo(apiContext.Context(), client, url, apiContext.Response)
		if lastError == nil {
			return nil
		}
	}

	return lastError
}

func (i *indexDownload) writeInfo(ctx context.Context, client *http.Client, url string, rw http.ResponseWriter) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	info, err := helm.InfoFromTarball(resp.Body)
	if err != nil {
		return err
	}

	rw.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(rw).Encode(info)
}

type repoDef struct {
	typedata *metav1.TypeMeta
	metadata *metav1.ObjectMeta
	spec     *v1.RepoSpec
	status   *v1.RepoStatus
}

func (i *indexDownload) getRepo(apiContext *types.APIRequest) (repoDef, error) {
	if isClusterRepo(apiContext.Type) {
		cr, err := i.clusterRepos.Get(apiContext.Name, metav1.GetOptions{})
		if err != nil {
			return repoDef{}, err
		}
		return repoDef{
			typedata: &cr.TypeMeta,
			metadata: &cr.ObjectMeta,
			spec:     &cr.Spec,
			status:   &cr.Status,
		}, nil
	}

	cr, err := i.repos.Get(apiContext.Namespace, apiContext.Name, metav1.GetOptions{})
	if err != nil {
		return repoDef{}, err
	}
	return repoDef{
		typedata: &cr.TypeMeta,
		metadata: &cr.ObjectMeta,
		spec:     &cr.Spec,
		status:   &cr.Status,
	}, nil
}

func (i *indexDownload) getIndex(apiContext *types.APIRequest) ([]byte, error) {
	repo, err := i.getRepo(apiContext)
	if err != nil {
		return nil, err
	}

	cm, err := i.configMaps.Get(repo.status.IndexConfigMapNamespace, repo.status.IndexConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if len(cm.OwnerReferences) == 0 || cm.OwnerReferences[0].UID != repo.metadata.UID {
		return nil, validation.Unauthorized
	}

	gz, err := gzip.NewReader(bytes.NewBuffer(cm.BinaryData["content"]))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	return ioutil.ReadAll(gz)
}
