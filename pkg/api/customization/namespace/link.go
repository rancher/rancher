package namespace

import (
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/resourcelink"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	printers2 "k8s.io/cli-runtime/pkg/printers"
	"k8s.io/kubernetes/pkg/printers"
)

var ExportPrinters = map[string]printers.ResourcePrinter{
	"json": &printers2.JSONPrinter{},
	"yaml": &printers2.YAMLPrinter{},
}

func NewLinkHandler(next types.RequestHandler, manager *clustermanager.Manager) types.RequestHandler {

	lh := &yamlLinkHandler{
		next:              next,
		clusterManagement: manager,
	}

	return lh.LinkHandler
}

type yamlLinkHandler struct {
	next              types.RequestHandler
	clusterManagement *clustermanager.Manager
}

func (s *yamlLinkHandler) callNext(apiContext *types.APIContext, next types.RequestHandler) error {
	if s.next != nil {
		return s.next(apiContext, next)
	} else if next != nil {
		return next(apiContext, nil)
	}

	return httperror.NewAPIError(httperror.NotFound, "link not found")
}

func (s *yamlLinkHandler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	if apiContext.Link != "yaml" {
		return s.callNext(apiContext, next)
	}

	clusterName := s.clusterManagement.ClusterName(apiContext)

	userContext, err := s.clusterManagement.UserContext(clusterName)
	if err != nil {
		return err
	}

	ns := apiContext.ID
	result := &unstructured.UnstructuredList{}
	result.SetAPIVersion("v1")
	result.SetKind("List")

	resources := apiContext.Request.URL.Query()["resource"]
	toExportResourceMappings := getResourcePrefixMap(resources)

	for kind, prefix := range toExportResourceMappings {

		req := userContext.UnversionedClient.Get().Prefix(prefix).Namespace(ns).Resource(kind)
		for k, v := range apiContext.Request.URL.Query() {
			req.Param(k, strings.Join(v, ","))
		}
		for k, v := range apiContext.Request.Header {
			if k == "Authorization" {
				continue
			}
			req.SetHeader(k, v...)
		}
		req.SetHeader("Accept", "*/*")

		r, err := req.Do(apiContext.Request.Context()).Get()
		if err != nil {
			if e, ok := err.(*apierrors.StatusError); ok && e.Status().Code == 403 {
				continue
			}
			return err
		}

		if list, ok := r.(*unstructured.UnstructuredList); ok {
			for _, item := range list.Items {
				if len(item.GetOwnerReferences()) == 0 {
					result.Items = append(result.Items, item)
				}
			}
		}
	}

	printer := ExportPrinters["json"]
	apiContext.Response.Header().Set("content-type", "application/json")

	if apiContext.Request.Header.Get("Accept") == "application/yaml" {
		printer = ExportPrinters["yaml"]
		apiContext.Response.Header().Set("content-type", "application/yaml")
	}

	return printer.PrintObj(result, apiContext.Response)
}

//getResourcePrefixMap converts resource path like `/api/v1/pods` to kind-prefix mappings
func getResourcePrefixMap(resources []string) map[string]string {
	if len(resources) == 0 {
		return resourcelink.ExportResourcePrefixMappings
	}
	m := map[string]string{}
	for _, r := range resources {
		idx := strings.LastIndex(r, "/")
		if idx == -1 {
			m[r] = ""
		} else {
			m[r[idx+1:]] = r[:idx]
		}

	}
	return m
}
