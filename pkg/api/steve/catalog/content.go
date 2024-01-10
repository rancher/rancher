package catalog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/catalogv2/content"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"helm.sh/helm/v3/pkg/repo"
)

// contentDownload implements the http handler interface.
type contentDownload struct {
	// contentManager struct functions helps in retrieving information
	// related to helm repositories such as chart asset,icon,info, helm repo index file
	// suitable to the current cluster and logs
	contentManager *content.Manager
}

// ServeHTTP is the main entry point for Apps & MarketPlace content service.
// It parses the request into apicontext to determine the type of content that
// needs to be served. The type of content being served is determined by the
// link field of the API context. It then calls the appropriate function of
// contentManager to serve the information.
func (i *contentDownload) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Get the APIContext from the current request's context. This APIContext
	// encapsulates the details of the API request, which will be used to
	// determine the necessary operation and respond accordingly.
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

// serveIndex retrieves the index file from the Helm repository, translates the URLs based on the current domain name,
// and sends the index file to the client.
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

// serveInfo provides detailed information about a specific chart
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

// serveIcon retrieves and serves the icon for a specific chart.
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
	if chart != nil {
		_, err = io.Copy(rw, chart)
		setIconHeaders(rw, suffix)
		defer chart.Close()
	}

	return err
}

// setIconHeaders sets headers necessary for the correct display of the icon image in most browsers.
func setIconHeaders(rw http.ResponseWriter, suffix string) {
	if suffix == ".svg" {
		rw.Header().Set("Content-Type", "image/svg+xml")
	}
	rw.Header().Set("Cache-Control", "max-age=31536000, public")
	rw.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	rw.Header().Set("X-Content-Type-Options", "nosniff")
}

// serveChart retrieves and serves a specific chart.
// The chart and its version are determined by the query parameters of the API request.
func (i *contentDownload) serveChart(apiContext *types.APIRequest, rw http.ResponseWriter, req *http.Request) error {
	query := apiContext.Request.URL.Query()
	chartName := query.Get("chartName")
	version := query.Get("version")

	if chartName == "" {
		return validation.NotFound
	}

	namespace, name := nsAndName(apiContext)
	chart, err := i.contentManager.Chart(namespace, name, chartName, version, true)
	if err != nil {
		return err
	}
	defer chart.Close()

	rw.Header().Set("Content-Type", "application/gzip")
	rw.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-%s.tgz\"", chartName, version))
	_, err = io.Copy(rw, chart)
	return err
}

// getIndex retrieves the index file from the Helm repository.
// By default, the index file contains versions filtered by rancher version and the local cluster's k8s version;
// If "skipFilter" is set to "true" in the API request, the index file will contain all versions for all charts;
// if "k8sVersion" is set, the index file will contain versions filtered by rancher version and the k8s version.
func (i *contentDownload) getIndex(apiContext *types.APIRequest) (*repo.IndexFile, error) {
	namespace, name := nsAndName(apiContext)
	query := apiContext.Request.URL.Query()
	rawValue := query.Get("skipFilter")
	skipFilter := strings.ToLower(rawValue) == "true"
	targetClusterVersion := query.Get("k8sVersion")
	return i.contentManager.Index(namespace, name, targetClusterVersion, skipFilter)
}

// nsAndName returns the namespace and name from the API context. If the
// API context corresponds to the cluster repository, the namespace is an empty string.
func nsAndName(apiContext *types.APIRequest) (string, string) {
	if isClusterRepo(apiContext.Type) {
		return "", apiContext.Name
	}
	return apiContext.Namespace, apiContext.Name
}
