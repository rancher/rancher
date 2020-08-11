package catalog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/catalogv2/content"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"helm.sh/helm/v3/pkg/repo"
)

type contentDownload struct {
	contentManager *content.Manager
}

func (i *contentDownload) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	apiContext := types.GetAPIContext(req.Context())
	switch apiContext.Link {
	case "index":
		if err := i.serveIndex(apiContext, rw, req); err != nil {
			apiContext.WriteError(err)
		}
	case "info":
		if err := i.serveInfo(apiContext, rw, req); err != nil {
			apiContext.WriteError(err)
		}
	case "chart":
		if err := i.serveChart(apiContext, rw, req); err != nil {
			apiContext.WriteError(err)
		}
	case "icon":
		if err := i.serveIcon(apiContext, rw, req); err != nil {
			apiContext.WriteError(err)
		}
	}
}

func (i *contentDownload) serveIndex(apiContext *types.APIRequest, rw http.ResponseWriter, req *http.Request) error {
	index, err := i.getIndex(apiContext)
	if err != nil {
		return err
	}

	u, err := url.Parse(apiContext.URLBuilder.Current())
	if err != nil {
		return err
	}

	if err := content.TranslateURLs(u, index); err != nil {
		return err
	}

	rw.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(rw).Encode(index)
}

func (i *contentDownload) serveInfo(apiContext *types.APIRequest, rw http.ResponseWriter, req *http.Request) error {
	query := apiContext.Request.URL.Query()
	chartName := query.Get("chartName")
	version := query.Get("version")

	if chartName == "" {
		return validation.NotFound
	}

	namespace, name := nsAndName(apiContext)
	info, err := i.contentManager.Info(namespace, name, chartName, version)
	if err != nil {
		return err
	}

	rw.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(rw).Encode(info)
}

func (i *contentDownload) serveIcon(apiContext *types.APIRequest, rw http.ResponseWriter, req *http.Request) error {
	query := apiContext.Request.URL.Query()
	chartName := query.Get("chartName")
	version := query.Get("version")

	if chartName == "" {
		return validation.NotFound
	}

	namespace, name := nsAndName(apiContext)
	chart, suffix, err := i.contentManager.Icon(namespace, name, chartName, version)
	if err != nil {
		return err
	}
	defer chart.Close()

	if suffix == ".svg" {
		rw.Header().Set("Content-Type", "image/svg+xml")
	}
	rw.Header().Set("Cache-Control", "max-age=31536000, public")
	_, err = io.Copy(rw, chart)
	return err
}

func (i *contentDownload) serveChart(apiContext *types.APIRequest, rw http.ResponseWriter, req *http.Request) error {
	query := apiContext.Request.URL.Query()
	chartName := query.Get("chartName")
	version := query.Get("version")

	if chartName == "" {
		return validation.NotFound
	}

	namespace, name := nsAndName(apiContext)
	chart, err := i.contentManager.Chart(namespace, name, chartName, version)
	if err != nil {
		return err
	}
	defer chart.Close()

	rw.Header().Set("Content-Type", "application/gzip")
	rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-%s.tgz\"", chartName, version))
	_, err = io.Copy(rw, chart)
	return err
}

func (i *contentDownload) getIndex(apiContext *types.APIRequest) (*repo.IndexFile, error) {
	return i.contentManager.Index(nsAndName(apiContext))
}

func nsAndName(apiContext *types.APIRequest) (string, string) {
	if isClusterRepo(apiContext.Type) {
		return "", apiContext.Name
	}
	return apiContext.Namespace, apiContext.Name
}
